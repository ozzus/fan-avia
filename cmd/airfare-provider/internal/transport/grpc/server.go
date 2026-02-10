package grpc

import (
	"context"
	"errors"
	"strings"

	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/application/service"
	derr "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/ports"
	airfarev1 "github.com/ozzus/fan-avia/protos/gen/go/airfare/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type serverAPI struct {
	airfarev1.UnimplementedAirfareProviderServiceServer
	log     *zap.Logger
	service *service.AirfareService
}

func Register(gRPCServer *grpc.Server, log *zap.Logger, airfareService *service.AirfareService) {
	airfarev1.RegisterAirfareProviderServiceServer(gRPCServer, &serverAPI{
		log:     log,
		service: airfareService,
	})
}

func (s *serverAPI) GetPricesForRules(ctx context.Context, req *airfarev1.GetPricesForRulesRequest) (*airfarev1.GetPricesForRulesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	if strings.TrimSpace(req.GetOriginIata()) == "" {
		return nil, status.Error(codes.InvalidArgument, "origin is required")
	}

	if strings.TrimSpace(req.GetDestinationIata()) == "" {
		return nil, status.Error(codes.InvalidArgument, "destination is required")
	}

	if req.GetTopN() == 0 {
		return nil, status.Error(codes.InvalidArgument, "top_n must be greater than 0")
	}

	if len(req.GetRules()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "rules must not be empty")
	}

	for i, rule := range req.GetRules() {
		if rule == nil {
			return nil, status.Errorf(codes.InvalidArgument, "rules[%d] is required", i)
		}

		if rule.GetType() == airfarev1.RuleType_RULE_TYPE_UNSPECIFIED {
			return nil, status.Errorf(codes.InvalidArgument, "rules[%d].type must be set", i)
		}

		if err := validateTimeConstraint(rule, i); err != nil {
			s.log.Warn("validation failed", zap.Error(err))
			return nil, err
		}
	}

	resp := &airfarev1.GetPricesForRulesResponse{
		Results: make([]*airfarev1.RuleResult, 0, len(req.GetRules())),
	}

	for _, rule := range req.GetRules() {
		resp.Results = append(resp.Results, &airfarev1.RuleResult{
			RuleId:  rule.GetType().String(),
			Options: stubOptions(req.GetTopN()),
		})
	}

	return resp, nil
}

func (s *serverAPI) GetAirfareByMatch(ctx context.Context, req *airfarev1.GetAirfareByMatchRequest) (*airfarev1.GetAirfareByMatchResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.GetMatchId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "match_id must be positive")
	}
	if strings.TrimSpace(req.GetOriginIata()) == "" {
		return nil, status.Error(codes.InvalidArgument, "origin_iata is required")
	}

	result, err := s.service.GetAirfareByMatch(ctx, req.GetMatchId(), req.GetOriginIata())
	if err != nil {
		return nil, mapGetAirfareByMatchError(err)
	}

	resp := &airfarev1.GetAirfareByMatchResponse{
		MatchId:     result.MatchID,
		TicketsLink: result.TicketsLink,
		Slots:       make([]*airfarev1.FareSlot, 0, len(result.Slots)),
	}

	for _, slot := range result.Slots {
		resp.Slots = append(resp.Slots, &airfarev1.FareSlot{
			Slot:      mapSlotKind(slot.Kind),
			Direction: mapDirection(slot.Direction),
			Date:      slot.DateUTC.Format("2006-01-02"),
			Prices:    slot.Prices,
		})
	}

	return resp, nil
}

func validateTimeConstraint(rule *airfarev1.Rule, idx int) error {
	tc := rule.GetTimeConstraint()
	switch rule.GetType() {
	case airfarev1.RuleType_RULE_TYPE_OUT_ARRIVE_BY:
		if tc == nil || tc.GetNotAfter() == nil || tc.GetNotBefore() != nil {
			return status.Errorf(codes.InvalidArgument, "rules[%d].time_constraint must have only not_after for RULE_OUT_ARRIVE_BY", idx)
		}
	case airfarev1.RuleType_RULE_TYPE_RET_DEPART_AFTER:
		if tc == nil || tc.GetNotBefore() == nil || tc.GetNotAfter() != nil {
			return status.Errorf(codes.InvalidArgument, "rules[%d].time_constraint must have only not_before for RULE_RET_DEPART_AFTER", idx)
		}
	default:
		if tc != nil {
			return status.Errorf(codes.InvalidArgument, "rules[%d].time_constraint must be empty for %s", idx, rule.GetType().String())
		}
	}

	return nil
}

func stubOptions(topN uint32) []*airfarev1.PriceOption {
	options := []*airfarev1.PriceOption{
		{Price: 10000, Currency: "RUB", Deeplink: ""},
		{Price: 12000, Currency: "RUB", Deeplink: ""},
		{Price: 15000, Currency: "RUB", Deeplink: ""},
	}

	if topN == 0 {
		return nil
	}

	if int(topN) < len(options) {
		return options[:int(topN)]
	}

	return options
}

func mapGetAirfareByMatchError(err error) error {
	switch {
	case errors.Is(err, derr.ErrInvalidOrigin):
		return status.Error(codes.InvalidArgument, "origin_iata is invalid")
	case errors.Is(err, derr.ErrMatchNotFound):
		return status.Error(codes.NotFound, "match not found")
	case errors.Is(err, derr.ErrSourceTemporary):
		return status.Error(codes.Unavailable, "source temporarily unavailable")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

func mapSlotKind(kind ports.SlotKind) airfarev1.FareSlotType {
	switch kind {
	case ports.SlotOutDMinus2:
		return airfarev1.FareSlotType_FARE_SLOT_OUT_D_MINUS_2
	case ports.SlotOutDMinus1:
		return airfarev1.FareSlotType_FARE_SLOT_OUT_D_MINUS_1
	case ports.SlotOutD0ArriveBy:
		return airfarev1.FareSlotType_FARE_SLOT_OUT_D0_ARRIVE_BY
	case ports.SlotRetD0DepartAfter:
		return airfarev1.FareSlotType_FARE_SLOT_RET_D0_DEPART_AFTER
	case ports.SlotRetDPlus1:
		return airfarev1.FareSlotType_FARE_SLOT_RET_D_PLUS_1
	case ports.SlotRetDPlus2:
		return airfarev1.FareSlotType_FARE_SLOT_RET_D_PLUS_2
	default:
		return airfarev1.FareSlotType_FARE_SLOT_UNSPECIFIED
	}
}

func mapDirection(direction ports.Direction) airfarev1.FareDirection {
	switch direction {
	case ports.DirectionOut:
		return airfarev1.FareDirection_FARE_DIRECTION_OUTBOUND
	case ports.DirectionRet:
		return airfarev1.FareDirection_FARE_DIRECTION_RETURN
	default:
		return airfarev1.FareDirection_FARE_DIRECTION_UNSPECIFIED
	}
}
