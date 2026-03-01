package service

import (
	"context"
	"errors"
	"strings"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/ports"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type AirfareService struct {
	log         *zap.Logger
	matchReader ports.MatchReader
	fareSource  ports.FareSource
	cache       ports.AirfareCache
	cacheTTL    time.Duration
	windows     MatchDayWindowPolicy
}

type MatchDayWindowPolicy struct {
	OutStrictEarliestBefore time.Duration
	OutStrictLatestBefore   time.Duration
	OutSoft1EarliestBefore  time.Duration
	OutSoft1LatestBefore    time.Duration
	OutSoft2EarliestBefore  time.Duration
	OutSoft2LatestBefore    time.Duration
	RetStrictNotBeforeAfter time.Duration
	RetSoft1NotBeforeAfter  time.Duration
	RetSoft2NotBeforeAfter  time.Duration
}

type fareSearchAttempt struct {
	level  ports.WindowLevel
	search ports.FareSearch
}

func DefaultMatchDayWindowPolicy() MatchDayWindowPolicy {
	return MatchDayWindowPolicy{
		OutStrictEarliestBefore: 4 * time.Hour,
		OutStrictLatestBefore:   2 * time.Hour,
		OutSoft1EarliestBefore:  8 * time.Hour,
		OutSoft1LatestBefore:    0,
		OutSoft2EarliestBefore:  24 * time.Hour,
		OutSoft2LatestBefore:    0,
		RetStrictNotBeforeAfter: 4 * time.Hour,
		RetSoft1NotBeforeAfter:  2 * time.Hour,
		RetSoft2NotBeforeAfter:  0,
	}
}

func NewAirfareService(
	log *zap.Logger,
	matchReader ports.MatchReader,
	fareSource ports.FareSource,
	cache ports.AirfareCache,
	cacheTTL time.Duration,
	windows MatchDayWindowPolicy,
) *AirfareService {
	if log == nil {
		log = zap.NewNop()
	}

	return &AirfareService{
		log:         log,
		matchReader: matchReader,
		fareSource:  fareSource,
		cache:       cache,
		cacheTTL:    cacheTTL,
		windows:     windows.normalized(),
	}
}

func (w MatchDayWindowPolicy) normalized() MatchDayWindowPolicy {
	defaults := DefaultMatchDayWindowPolicy()

	w.OutStrictEarliestBefore = normalizeDuration(w.OutStrictEarliestBefore, defaults.OutStrictEarliestBefore)
	w.OutStrictLatestBefore = normalizeDuration(w.OutStrictLatestBefore, defaults.OutStrictLatestBefore)
	w.OutStrictEarliestBefore, w.OutStrictLatestBefore = normalizeBeforeWindow(w.OutStrictEarliestBefore, w.OutStrictLatestBefore)

	w.OutSoft1EarliestBefore = normalizeDuration(w.OutSoft1EarliestBefore, defaults.OutSoft1EarliestBefore)
	w.OutSoft1LatestBefore = normalizeDuration(w.OutSoft1LatestBefore, defaults.OutSoft1LatestBefore)
	w.OutSoft1EarliestBefore, w.OutSoft1LatestBefore = normalizeBeforeWindow(w.OutSoft1EarliestBefore, w.OutSoft1LatestBefore)

	w.OutSoft2EarliestBefore = normalizeDuration(w.OutSoft2EarliestBefore, defaults.OutSoft2EarliestBefore)
	w.OutSoft2LatestBefore = normalizeDuration(w.OutSoft2LatestBefore, defaults.OutSoft2LatestBefore)
	w.OutSoft2EarliestBefore, w.OutSoft2LatestBefore = normalizeBeforeWindow(w.OutSoft2EarliestBefore, w.OutSoft2LatestBefore)

	w.RetStrictNotBeforeAfter = normalizeDuration(w.RetStrictNotBeforeAfter, defaults.RetStrictNotBeforeAfter)
	w.RetSoft1NotBeforeAfter = normalizeDuration(w.RetSoft1NotBeforeAfter, defaults.RetSoft1NotBeforeAfter)
	w.RetSoft2NotBeforeAfter = normalizeDuration(w.RetSoft2NotBeforeAfter, defaults.RetSoft2NotBeforeAfter)

	return w
}

func normalizeDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value < 0 {
		return fallback
	}
	return value
}

func normalizeBeforeWindow(earliestBefore, latestBefore time.Duration) (time.Duration, time.Duration) {
	if earliestBefore < latestBefore {
		return latestBefore, earliestBefore
	}
	return earliestBefore, latestBefore
}

func (s *AirfareService) GetAirfareByMatch(ctx context.Context, matchID int64, originIATA string) (ports.AirfareByMatch, error) {
	const op = "service.GetAirfareByMatch"
	tracer := otel.Tracer("airfare-provider/service")
	ctx, span := tracer.Start(ctx, op)
	defer span.End()
	span.SetAttributes(
		attribute.Int64("airfare.match_id", matchID),
		attribute.String("airfare.origin_iata", strings.ToUpper(strings.TrimSpace(originIATA))),
	)

	logger := s.log.With(
		zap.String("op", op),
		zap.Int64("match_id", matchID),
		zap.String("origin_iata", originIATA),
	)

	if matchID <= 0 {
		logger.Warn("invalid match_id")
		span.SetStatus(otelcodes.Error, "invalid match_id")
		return ports.AirfareByMatch{}, derr.ErrMatchNotFound
	}
	if strings.TrimSpace(originIATA) == "" {
		logger.Warn("invalid origin_iata")
		span.SetStatus(otelcodes.Error, "invalid origin_iata")
		return ports.AirfareByMatch{}, derr.ErrInvalidOrigin
	}

	if s.cache != nil {
		cached, err := s.cache.GetByMatchAndOrigin(ctx, matchID, originIATA)
		if err == nil {
			logger.Info("airfare cache hit")
			span.AddEvent("airfare.cache.hit")
			return cached, nil
		}
		if errors.Is(err, derr.ErrAirfareNotFound) {
			logger.Info("airfare cache miss")
			span.AddEvent("airfare.cache.miss")
		}
		if !errors.Is(err, derr.ErrAirfareNotFound) {
			logger.Warn("redis cache read failed", zap.Error(err))
			span.RecordError(err)
		}
	}

	match, err := s.matchReader.GetMatch(ctx, matchID)
	if err != nil {
		logger.Warn("failed to load match snapshot", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to load match snapshot")
		return ports.AirfareByMatch{}, err
	}

	destinationIATA := strings.ToUpper(strings.TrimSpace(match.DestinationIATA))
	normalizedOrigin := strings.ToUpper(strings.TrimSpace(originIATA))
	if normalizedOrigin == destinationIATA {
		logger.Warn(
			"invalid route: origin equals destination",
			zap.String("destination_iata", destinationIATA),
		)
		span.SetStatus(otelcodes.Error, "invalid route")
		return ports.AirfareByMatch{}, derr.ErrInvalidRoute
	}

	kickoffUTC := match.KickoffUTC.UTC()
	if match.KickoffUTC.Location() != time.UTC {
		logger.Info(
			"normalized kickoff to utc",
			zap.Time("kickoff_original", match.KickoffUTC),
			zap.Time("kickoff_utc", kickoffUTC),
		)
	}

	result := ports.AirfareByMatch{
		MatchID:     match.MatchID,
		TicketsLink: match.TicketsLink,
		Slots:       buildDefaultSlots(kickoffUTC),
	}

	if s.fareSource != nil {
		sourceCalls := 0
		sourceFailures := 0
		for i := range result.Slots {
			attempts := s.buildFareSearchAttempts(result.Slots[i], normalizedOrigin, destinationIATA, kickoffUTC)
			selectedLevel := ports.WindowLevelStrict
			selectedPrices := []int64{}

			for attemptIdx, attempt := range attempts {
				sourceCalls++
				prices, err := s.fareSource.GetPrices(ctx, attempt.search)
				if err != nil {
					sourceFailures++
					logger.Warn(
						"failed to fetch prices for slot",
						zap.String("slot_kind", slotKindToString(result.Slots[i].Kind)),
						zap.String("window_level", windowLevelToString(attempt.level)),
						zap.Error(err),
					)
					span.AddEvent(
						"airfare.source.slot_error",
						trace.WithAttributes(
							attribute.String("airfare.slot_kind", slotKindToString(result.Slots[i].Kind)),
							attribute.String("airfare.window_level", windowLevelToString(attempt.level)),
						),
					)
					span.RecordError(err)
					continue
				}

				selectedLevel = attempt.level
				selectedPrices = prices
				if selectedPrices == nil {
					selectedPrices = []int64{}
				}

				if len(selectedPrices) > 0 || attemptIdx == len(attempts)-1 {
					break
				}
			}

			result.Slots[i].WindowLevel = selectedLevel
			result.Slots[i].Prices = selectedPrices
		}
		if sourceCalls > 0 && sourceFailures == sourceCalls {
			span.SetStatus(otelcodes.Error, "all source calls failed")
			return ports.AirfareByMatch{}, derr.ErrSourceTemporary
		}
	}

	if s.cache != nil {
		if err := s.cache.SetByMatchAndOrigin(ctx, matchID, originIATA, result, s.cacheTTL); err != nil {
			logger.Warn("redis cache write failed", zap.Error(err))
			span.RecordError(err)
		}
	}

	span.SetAttributes(attribute.Int("airfare.slots_count", len(result.Slots)))
	span.SetStatus(otelcodes.Ok, "ok")
	logger.Info("airfare slots built", zap.Int("slots_count", len(result.Slots)))
	return result, nil
}

func buildDefaultSlots(kickoffUTC time.Time) []ports.FareSlot {
	day := time.Date(kickoffUTC.Year(), kickoffUTC.Month(), kickoffUTC.Day(), 0, 0, 0, 0, time.UTC)

	return []ports.FareSlot{
		{Kind: ports.SlotOutDMinus2, Direction: ports.DirectionOut, DateUTC: day.AddDate(0, 0, -2), Prices: []int64{}, WindowLevel: ports.WindowLevelStrict},
		{Kind: ports.SlotOutDMinus1, Direction: ports.DirectionOut, DateUTC: day.AddDate(0, 0, -1), Prices: []int64{}, WindowLevel: ports.WindowLevelStrict},
		{Kind: ports.SlotOutD0ArriveBy, Direction: ports.DirectionOut, DateUTC: day, Prices: []int64{}, WindowLevel: ports.WindowLevelStrict},
		{Kind: ports.SlotRetD0DepartAfter, Direction: ports.DirectionRet, DateUTC: day, Prices: []int64{}, WindowLevel: ports.WindowLevelStrict},
		{Kind: ports.SlotRetDPlus1, Direction: ports.DirectionRet, DateUTC: day.AddDate(0, 0, 1), Prices: []int64{}, WindowLevel: ports.WindowLevelStrict},
		{Kind: ports.SlotRetDPlus2, Direction: ports.DirectionRet, DateUTC: day.AddDate(0, 0, 2), Prices: []int64{}, WindowLevel: ports.WindowLevelStrict},
	}
}

func (s *AirfareService) buildFareSearchAttempts(slot ports.FareSlot, originIATA, destinationIATA string, kickoffUTC time.Time) []fareSearchAttempt {
	base := ports.FareSearch{
		DateUTC: slot.DateUTC,
	}

	if slot.Direction == ports.DirectionOut {
		base.OriginIATA = strings.ToUpper(strings.TrimSpace(originIATA))
		base.DestinationIATA = strings.ToUpper(strings.TrimSpace(destinationIATA))
	} else {
		base.OriginIATA = strings.ToUpper(strings.TrimSpace(destinationIATA))
		base.DestinationIATA = strings.ToUpper(strings.TrimSpace(originIATA))
	}

	switch slot.Kind {
	case ports.SlotOutD0ArriveBy:
		return s.outboundDayMatchAttempts(base, kickoffUTC)
	case ports.SlotRetD0DepartAfter:
		return s.returnDayMatchAttempts(base, kickoffUTC)
	}

	return []fareSearchAttempt{
		{
			level:  ports.WindowLevelStrict,
			search: base,
		},
	}
}

func (s *AirfareService) outboundDayMatchAttempts(base ports.FareSearch, kickoffUTC time.Time) []fareSearchAttempt {
	windows := []struct {
		level          ports.WindowLevel
		earliestBefore time.Duration
		latestBefore   time.Duration
	}{
		{
			level:          ports.WindowLevelStrict,
			earliestBefore: s.windows.OutStrictEarliestBefore,
			latestBefore:   s.windows.OutStrictLatestBefore,
		},
		{
			level:          ports.WindowLevelSoft1,
			earliestBefore: s.windows.OutSoft1EarliestBefore,
			latestBefore:   s.windows.OutSoft1LatestBefore,
		},
		{
			level:          ports.WindowLevelSoft2,
			earliestBefore: s.windows.OutSoft2EarliestBefore,
			latestBefore:   s.windows.OutSoft2LatestBefore,
		},
	}

	attempts := make([]fareSearchAttempt, 0, len(windows))
	for _, window := range windows {
		search := base
		arriveNotBefore := kickoffUTC.Add(-window.earliestBefore)
		arriveNotLater := kickoffUTC.Add(-window.latestBefore)
		search.ArriveNotBeforeUTC = &arriveNotBefore
		search.ArriveNotLaterUTC = &arriveNotLater
		attempts = append(attempts, fareSearchAttempt{
			level:  window.level,
			search: search,
		})
	}

	return attempts
}

func (s *AirfareService) returnDayMatchAttempts(base ports.FareSearch, kickoffUTC time.Time) []fareSearchAttempt {
	windows := []struct {
		level          ports.WindowLevel
		notBeforeAfter time.Duration
	}{
		{
			level:          ports.WindowLevelStrict,
			notBeforeAfter: s.windows.RetStrictNotBeforeAfter,
		},
		{
			level:          ports.WindowLevelSoft1,
			notBeforeAfter: s.windows.RetSoft1NotBeforeAfter,
		},
		{
			level:          ports.WindowLevelSoft2,
			notBeforeAfter: s.windows.RetSoft2NotBeforeAfter,
		},
	}

	attempts := make([]fareSearchAttempt, 0, len(windows))
	for _, window := range windows {
		search := base
		limit := kickoffUTC.Add(window.notBeforeAfter)
		search.DepartNotBeforeUTC = &limit
		attempts = append(attempts, fareSearchAttempt{
			level:  window.level,
			search: search,
		})
	}

	return attempts
}

func slotKindToString(kind ports.SlotKind) string {
	switch kind {
	case ports.SlotOutDMinus2:
		return "OUT_D_MINUS_2"
	case ports.SlotOutDMinus1:
		return "OUT_D_MINUS_1"
	case ports.SlotOutD0ArriveBy:
		return "OUT_D0_ARRIVE_BY"
	case ports.SlotRetD0DepartAfter:
		return "RET_D0_DEPART_AFTER"
	case ports.SlotRetDPlus1:
		return "RET_D_PLUS_1"
	case ports.SlotRetDPlus2:
		return "RET_D_PLUS_2"
	default:
		return "UNKNOWN"
	}
}

func windowLevelToString(level ports.WindowLevel) string {
	switch level {
	case ports.WindowLevelStrict:
		return "STRICT"
	case ports.WindowLevelSoft1:
		return "SOFT_1"
	case ports.WindowLevelSoft2:
		return "SOFT_2"
	default:
		return "UNKNOWN"
	}
}
