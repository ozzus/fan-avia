package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/application/service"
	derr "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/ports"
	airfarev1 "github.com/ozzus/fan-avia/protos/gen/go/airfare/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcTestMatchReader struct {
	match ports.MatchSnapshot
	err   error
}

func (m grpcTestMatchReader) GetMatch(ctx context.Context, matchID int64) (ports.MatchSnapshot, error) {
	if m.err != nil {
		return ports.MatchSnapshot{}, m.err
	}
	return m.match, nil
}

type grpcTestFareSource struct{}

func (grpcTestFareSource) GetPrices(ctx context.Context, search ports.FareSearch) ([]int64, error) {
	return []int64{1234}, nil
}

func TestGetAirfareByMatch_InvalidOrigin(t *testing.T) {
	srv := &serverAPI{service: service.NewAirfareService(zap.NewNop(), grpcTestMatchReader{}, grpcTestFareSource{}, nil, 0)}

	_, err := srv.GetAirfareByMatch(context.Background(), &airfarev1.GetAirfareByMatchRequest{
		MatchId:    16114,
		OriginIata: " ",
	})
	if err == nil {
		t.Fatal("expected error for empty origin_iata")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unexpected code: got %v want %v", status.Code(err), codes.InvalidArgument)
	}
}

func TestGetAirfareByMatch_MapsNotFound(t *testing.T) {
	srv := &serverAPI{service: service.NewAirfareService(zap.NewNop(), grpcTestMatchReader{err: derr.ErrMatchNotFound}, grpcTestFareSource{}, nil, 0)}

	_, err := srv.GetAirfareByMatch(context.Background(), &airfarev1.GetAirfareByMatchRequest{
		MatchId:    16114,
		OriginIata: "MOW",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unexpected code: got %v want %v", status.Code(err), codes.NotFound)
	}
}

func TestGetAirfareByMatch_Success(t *testing.T) {
	kickoff := time.Date(2026, 2, 27, 19, 30, 0, 0, time.UTC)
	srv := &serverAPI{
		service: service.NewAirfareService(
			zap.NewNop(),
			grpcTestMatchReader{match: ports.MatchSnapshot{
				MatchID:         16114,
				KickoffUTC:      kickoff,
				DestinationIATA: "LED",
				TicketsLink:     "https://tickets.fc-zenit.ru/football/tickets/#zenit",
			}},
			grpcTestFareSource{},
			nil,
			0,
		),
	}

	resp, err := srv.GetAirfareByMatch(context.Background(), &airfarev1.GetAirfareByMatchRequest{
		MatchId:    16114,
		OriginIata: "MOW",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetMatchId() != 16114 {
		t.Fatalf("unexpected match_id: got %d", resp.GetMatchId())
	}
	if len(resp.GetSlots()) != 6 {
		t.Fatalf("unexpected slots count: got %d want 6", len(resp.GetSlots()))
	}
}
