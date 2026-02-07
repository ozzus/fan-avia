package grpc

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/application/service"
	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
	matchv1 "github.com/ozzus/fan-avia/protos/gen/go/match/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type serverAPI struct {
	matchv1.UnimplementedMatchAdapterServiceServer
	log     *zap.Logger
	service *service.MatchService
}

func Register(gRPCServer *grpc.Server, log *zap.Logger, matchService *service.MatchService) {
	matchv1.RegisterMatchAdapterServiceServer(gRPCServer, &serverAPI{log: log, service: matchService})
}

func (s *serverAPI) GetMatch(ctx context.Context, req *matchv1.GetMatchRequest) (*matchv1.GetMatchResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.GetMatchId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "match_id must be positive")
	}

	id := models.MatchID(fmt.Sprintf("%d", req.GetMatchId()))
	m, err := s.service.GetMatch(ctx, id)
	if err != nil {
		s.log.Error("GetMatch failed", zap.Int64("match_id", req.GetMatchId()), zap.Error(err))
		return nil, mapGetMatchError(err)
	}

	matchID, err := strconv.ParseInt(string(m.ID), 10, 64)
	if err != nil {
		s.log.Error("failed to parse match id", zap.String("match_id", string(m.ID)), zap.Error(err))
		return nil, status.Error(codes.Internal, "invalid match id in storage")
	}

	return &matchv1.GetMatchResponse{
		Match: &matchv1.Match{
			MatchId:                matchID,
			KickoffUtc:             timestamppb.New(m.KickoffUTC),
			City:                   m.City,
			Stadium:                m.Stadium,
			DestinationAirportIata: m.DestinationIATA,
			ClubHomeId:             m.HomeTeam,
			ClubAwayId:             m.AwayTeam,
		},
	}, nil
}

func mapGetMatchError(err error) error {
	switch {
	case errors.Is(err, derr.ErrMatchNotFound):
		return status.Error(codes.NotFound, "match not found")
	case errors.Is(err, derr.ErrSourceUnavailable):
		return status.Error(codes.Unavailable, "match source unavailable")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
