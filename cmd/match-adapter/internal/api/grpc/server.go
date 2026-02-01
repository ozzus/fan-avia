package grpc

import (
	"context"

	matchv1 "github.com/ozzus/fan-avia/protos/gen/go/match/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type serverAPI struct {
	matchv1.UnimplementedMatchAdapterServiceServer
	log *zap.Logger
}

func Register(gRPCServer *grpc.Server, log *zap.Logger) {
	matchv1.RegisterMatchAdapterServiceServer(gRPCServer, &serverAPI{log: log})
}

func (s *serverAPI) GetMatch(ctx context.Context, req *matchv1.GetMatchRequest) (*matchv1.GetMatchResponse, error) {
	if req == nil {
		s.log.Warn("GetMatch called with nil request")
		return &matchv1.GetMatchResponse{}, nil
	}

	s.log.Info("GetMatch stub", zap.Int64("match_id", req.GetMatchId()))
	return &matchv1.GetMatchResponse{}, nil
}
