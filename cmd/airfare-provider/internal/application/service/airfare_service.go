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
}

func NewAirfareService(log *zap.Logger, matchReader ports.MatchReader, fareSource ports.FareSource, cache ports.AirfareCache, cacheTTL time.Duration) *AirfareService {
	if log == nil {
		log = zap.NewNop()
	}

	return &AirfareService{
		log:         log,
		matchReader: matchReader,
		fareSource:  fareSource,
		cache:       cache,
		cacheTTL:    cacheTTL,
	}
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
		sourceFailures := 0
		for i := range result.Slots {
			search := s.buildFareSearch(result.Slots[i], normalizedOrigin, destinationIATA, kickoffUTC)
			prices, err := s.fareSource.GetPrices(ctx, search)
			if err != nil {
				sourceFailures++
				logger.Warn("failed to fetch prices for slot",
					zap.String("slot_kind", slotKindToString(result.Slots[i].Kind)),
					zap.Error(err),
				)
				span.AddEvent(
					"airfare.source.slot_error",
					trace.WithAttributes(attribute.String("airfare.slot_kind", slotKindToString(result.Slots[i].Kind))),
				)
				span.RecordError(err)
				continue
			}
			result.Slots[i].Prices = prices
		}
		if sourceFailures == len(result.Slots) {
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
		{Kind: ports.SlotOutDMinus2, Direction: ports.DirectionOut, DateUTC: day.AddDate(0, 0, -2), Prices: []int64{}},
		{Kind: ports.SlotOutDMinus1, Direction: ports.DirectionOut, DateUTC: day.AddDate(0, 0, -1), Prices: []int64{}},
		{Kind: ports.SlotOutD0ArriveBy, Direction: ports.DirectionOut, DateUTC: day, Prices: []int64{}},
		{Kind: ports.SlotRetD0DepartAfter, Direction: ports.DirectionRet, DateUTC: day, Prices: []int64{}},
		{Kind: ports.SlotRetDPlus1, Direction: ports.DirectionRet, DateUTC: day.AddDate(0, 0, 1), Prices: []int64{}},
		{Kind: ports.SlotRetDPlus2, Direction: ports.DirectionRet, DateUTC: day.AddDate(0, 0, 2), Prices: []int64{}},
	}
}

func (s *AirfareService) buildFareSearch(slot ports.FareSlot, originIATA, destinationIATA string, kickoffUTC time.Time) ports.FareSearch {
	search := ports.FareSearch{
		DateUTC: slot.DateUTC,
	}

	if slot.Direction == ports.DirectionOut {
		search.OriginIATA = strings.ToUpper(strings.TrimSpace(originIATA))
		search.DestinationIATA = strings.ToUpper(strings.TrimSpace(destinationIATA))
	} else {
		search.OriginIATA = strings.ToUpper(strings.TrimSpace(destinationIATA))
		search.DestinationIATA = strings.ToUpper(strings.TrimSpace(originIATA))
	}

	switch slot.Kind {
	case ports.SlotOutD0ArriveBy:
		limit := kickoffUTC.Add(-4 * time.Hour)
		search.ArriveNotLaterUTC = &limit
	case ports.SlotRetD0DepartAfter:
		limit := kickoffUTC.Add(2 * time.Hour)
		search.DepartNotBeforeUTC = &limit
	}

	return search
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
