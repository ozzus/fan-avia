package service

import (
	"context"
	"errors"
	"fmt"
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
