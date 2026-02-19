package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/ports"
	"go.uber.org/zap"
)

type MatchService struct {
	log      *zap.Logger
	source   ports.MatchSource
	resolver ports.CityIATAResolver
	repo     ports.MatchRepository
	cache    ports.MatchCache
	cacheTTL time.Duration
}

const (
	defaultUpcomingLimit = 10
	maxUpcomingLimit     = 100
)

func NewMatchService(log *zap.Logger, source ports.MatchSource, resolver ports.CityIATAResolver, repo ports.MatchRepository, cache ports.MatchCache, cacheTTL time.Duration) *MatchService {
	return &MatchService{
		log:      log,
		source:   source,
		resolver: resolver,
		repo:     repo,
		cache:    cache,
		cacheTTL: cacheTTL,
	}
}

func (s *MatchService) GetMatch(ctx context.Context, id models.MatchID) (models.Match, error) {
	const op = "service.GetMatch"

	logger := s.log.With(
		zap.String("op", op),
		zap.String("match_id", string(id)),
	)

	if s.cache != nil {
		match, err := s.cache.GetByID(ctx, id)
		if err == nil {
			logger.Debug("match loaded from redis cache")
			return match, nil
		}
		if !errors.Is(err, derr.ErrMatchNotFound) {
			logger.Warn("redis cache read failed", zap.Error(err))
		}
	}

	match, err := s.repo.GetByID(ctx, id)
	if err == nil {
		logger.Debug("match loaded from db")
		if s.cache != nil {
			if err := s.cache.Set(ctx, match, s.cacheTTL); err != nil {
				logger.Warn("redis cache write failed", zap.Error(err))
			}
		}
		return match, nil
	}
	if !errors.Is(err, derr.ErrMatchNotFound) {
		return models.Match{}, fmt.Errorf("%s: get match from repo: %w", op, err)
	}

	match, err = s.source.FetchByID(ctx, id)
	if err != nil {
		return models.Match{}, fmt.Errorf("%s: fetch match from source: %w", op, err)
	}

	if match.DestinationIATA == "" {
		iata, err := s.resolver.ResolveDestinationIATA(ctx, match.City)
		if err != nil {
			return models.Match{}, fmt.Errorf("%s: resolve destination iata: %w", op, err)
		}
		match.DestinationIATA = iata
	}

	if err := s.repo.Upsert(ctx, match); err != nil {
		return models.Match{}, fmt.Errorf("%s: upsert match: %w", op, err)
	}

	if s.cache != nil {
		if err := s.cache.Set(ctx, match, s.cacheTTL); err != nil {
			logger.Warn("redis cache write failed", zap.Error(err))
		}
	}

	logger.Info("match fetched from source and saved")
	return match, nil
}

func (s *MatchService) GetUpcomingMatches(ctx context.Context, limit int, clubID string) ([]models.Match, error) {
	const op = "service.GetUpcomingMatches"

	limit = normalizeUpcomingLimit(limit)
	clubID = strings.TrimSpace(clubID)

	matches, err := s.repo.GetUpcoming(ctx, limit, clubID)
	if err != nil {
		return nil, fmt.Errorf("%s: get upcoming matches from repo: %w", op, err)
	}

	return matches, nil
}

func (s *MatchService) SyncUpcomingMatches(ctx context.Context, from time.Time, to time.Time, limit int) (int, error) {
	const op = "service.SyncUpcomingMatches"

	if from.IsZero() {
		from = time.Now().UTC()
	}
	from = from.UTC()

	if to.IsZero() || !to.After(from) {
		to = from.Add(90 * 24 * time.Hour)
	}
	to = to.UTC()

	limit = normalizeUpcomingLimit(limit)

	logger := s.log.With(
		zap.String("op", op),
		zap.Time("from_utc", from),
		zap.Time("to_utc", to),
		zap.Int("limit", limit),
	)

	ids, err := s.source.FetchUpcomingIDs(ctx, from, to, limit)
	if err != nil {
		return 0, fmt.Errorf("%s: fetch upcoming ids: %w", op, err)
	}
	if len(ids) == 0 {
		logger.Info("no upcoming matches from source")
		return 0, nil
	}

	var saved int
	var failed int

	for _, id := range ids {
		match, err := s.source.FetchByID(ctx, id)
		if err != nil {
			if isContextErr(err) {
				return saved, err
			}
			failed++
			logger.Warn("failed to fetch match by id", zap.String("match_id", string(id)), zap.Error(err))
			continue
		}

		if match.DestinationIATA == "" {
			iata, err := s.resolver.ResolveDestinationIATA(ctx, match.City)
			if err != nil {
				if isContextErr(err) {
					return saved, err
				}
				failed++
				logger.Warn(
					"failed to resolve destination iata",
					zap.String("match_id", string(id)),
					zap.String("city", match.City),
					zap.Error(err),
				)
				continue
			}
			match.DestinationIATA = iata
		}

		if err := s.repo.Upsert(ctx, match); err != nil {
			if isContextErr(err) {
				return saved, err
			}
			failed++
			logger.Warn("failed to upsert match", zap.String("match_id", string(id)), zap.Error(err))
			continue
		}

		if s.cache != nil {
			if err := s.cache.Set(ctx, match, s.cacheTTL); err != nil {
				logger.Warn("redis cache write failed during sync", zap.String("match_id", string(id)), zap.Error(err))
			}
		}

		saved++
	}

	logger.Info(
		"upcoming matches sync finished",
		zap.Int("requested", len(ids)),
		zap.Int("saved", saved),
		zap.Int("failed", failed),
	)

	if saved == 0 && len(ids) > 0 {
		return 0, fmt.Errorf("%s: no matches synced", op)
	}

	return saved, nil
}

func normalizeUpcomingLimit(limit int) int {
	if limit <= 0 {
		return defaultUpcomingLimit
	}
	if limit > maxUpcomingLimit {
		return maxUpcomingLimit
	}
	return limit
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
