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
	maxUpcomingLimit     = 500
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
		if err := s.enrichDestinationFromCityOrClub(ctx, &match); err != nil {
			return models.Match{}, fmt.Errorf("%s: resolve destination iata: %w", op, err)
		}
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

	matches, err := s.repo.GetUpcoming(ctx, limit, clubID)
	if err != nil {
		return nil, fmt.Errorf("%s: get upcoming matches from repo: %w", op, err)
	}

	return matches, nil
}

func (s *MatchService) GetClubs(ctx context.Context) ([]models.Club, error) {
	const op = "service.GetClubs"

	clubs, err := s.repo.GetClubs(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: get clubs from repo: %w", op, err)
	}

	return clubs, nil
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
			if err := s.enrichDestinationFromCityOrClub(ctx, &match); err != nil {
				if isContextErr(err) {
					return saved, err
				}
				failed++
				logger.Warn(
					"failed to resolve destination iata",
					zap.String("match_id", string(id)),
					zap.String("city", match.City),
					zap.String("club_home_id", match.HomeTeam),
					zap.Error(err),
				)
				continue
			}
		}

		existing, err := s.repo.GetByID(ctx, match.ID)
		if err == nil {
			logMatchDiff(logger, existing, match)
		} else if !errors.Is(err, derr.ErrMatchNotFound) {
			logger.Warn(
				"failed to load existing match before upsert",
				zap.String("match_id", string(id)),
				zap.Error(err),
			)
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

func (s *MatchService) enrichDestinationFromCityOrClub(ctx context.Context, match *models.Match) error {
	city := strings.TrimSpace(match.City)
	if city != "" {
		iata, err := s.resolver.ResolveDestinationIATA(ctx, city)
		if err == nil {
			match.DestinationIATA = iata
			return nil
		}
		if !errors.Is(err, derr.ErrCityIATANotFound) {
			return err
		}
	}

	fallback, ok := fallbackByHomeClubID[match.HomeTeam]
	if !ok {
		return derr.ErrCityIATANotFound
	}

	if city == "" {
		match.City = fallback.city
	}
	match.DestinationIATA = fallback.iata

	return nil
}

type clubFallback struct {
	city string
	iata string
}

var fallbackByHomeClubID = map[string]clubFallback{
	"1":   {city: "Москва", iata: "MOW"},          // Спартак Москва
	"2":   {city: "Москва", iata: "MOW"},          // ПФК ЦСКА
	"3":   {city: "Санкт-Петербург", iata: "LED"}, // Зенит
	"4":   {city: "Казань", iata: "KZN"},          // Рубин
	"5":   {city: "Москва", iata: "MOW"},          // Локомотив
	"7":   {city: "Москва", iata: "MOW"},          // Динамо Москва
	"10":  {city: "Самара", iata: "KUF"},          // Крылья Советов
	"11":  {city: "Ростов-на-Дону", iata: "ROV"}, // Ростов
	"125": {city: "Каспийск", iata: "MCX"},       // Динамо Махачкала
	"444": {city: "Калининград", iata: "KGD"},    // Балтика
	"504": {city: "Оренбург", iata: "REN"},       // Оренбург
	"525": {city: "Сочи", iata: "AER"},           // Сочи
	"584": {city: "Краснодар", iata: "KRR"},      // Краснодар
	"702": {city: "Грозный", iata: "GRV"},        // Ахмат
	"704": {city: "Нижний Новгород", iata: "GOJ"}, // Пари НН
	"807": {city: "Самара", iata: "KUF"},         // Акрон
}

func logMatchDiff(logger *zap.Logger, oldMatch models.Match, newMatch models.Match) {
	diffFields := make([]string, 0, 8)

	if oldMatch.HomeTeam != newMatch.HomeTeam {
		diffFields = append(diffFields, "club_home_id")
	}
	if oldMatch.AwayTeam != newMatch.AwayTeam {
		diffFields = append(diffFields, "club_away_id")
	}
	if oldMatch.City != newMatch.City {
		diffFields = append(diffFields, "city")
	}
	if oldMatch.Stadium != newMatch.Stadium {
		diffFields = append(diffFields, "stadium")
	}
	if oldMatch.KickoffUTC.UTC() != newMatch.KickoffUTC.UTC() {
		diffFields = append(diffFields, "kickoff_utc")
	}
	if oldMatch.DestinationIATA != newMatch.DestinationIATA {
		diffFields = append(diffFields, "destination_iata")
	}
	if oldMatch.TicketsLink != newMatch.TicketsLink {
		diffFields = append(diffFields, "tickets_link")
	}

	if len(diffFields) == 0 {
		return
	}

	logger.Warn(
		"match snapshot changed by source sync",
		zap.String("match_id", string(newMatch.ID)),
		zap.Strings("diff_fields", diffFields),
		zap.String("old_kickoff_utc", oldMatch.KickoffUTC.UTC().Format(time.RFC3339)),
		zap.String("new_kickoff_utc", newMatch.KickoffUTC.UTC().Format(time.RFC3339)),
		zap.String("old_home_club_id", oldMatch.HomeTeam),
		zap.String("new_home_club_id", newMatch.HomeTeam),
		zap.String("old_away_club_id", oldMatch.AwayTeam),
		zap.String("new_away_club_id", newMatch.AwayTeam),
	)
}
