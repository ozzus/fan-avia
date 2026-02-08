package grpc

import (
	"context"
	"strings"

	airfarev1 "github.com/ozzus/fan-avia/protos/gen/go/airfare/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type serverAPI struct {
	airfarev1.UnimplementedAirfareProviderServer
	log *zap.Logger
}

func Register(gRPCServer *grpc.Server, log *zap.Logger) {
	airfarev1.RegisterAirfareProviderServer(gRPCServer, &serverAPI{log: log})
}

func (s *serverAPI) GetPricesForRules(ctx context.Context, req *airfarev1.GetPricesForRulesRequest) (*airfarev1.GetPricesForRulesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	if strings.TrimSpace(req.GetOrigin()) == "" {
		return nil, status.Error(codes.InvalidArgument, "origin is required")
	}

	if strings.TrimSpace(req.GetDestination()) == "" {
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

		if rule.GetType() == airfarev1.RuleType_RULE_UNSPECIFIED {
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

func validateTimeConstraint(rule *airfarev1.Rule, idx int) error {
	tc := rule.GetTimeConstraint()
	switch rule.GetType() {
	case airfarev1.RuleType_RULE_OUT_ARRIVE_BY:
		if tc == nil || tc.GetNotAfter() == nil || tc.GetNotBefore() != nil {
			return status.Errorf(codes.InvalidArgument, "rules[%d].time_constraint must have only not_after for RULE_OUT_ARRIVE_BY", idx)
		}
	case airfarev1.RuleType_RULE_RET_DEPART_AFTER:
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
