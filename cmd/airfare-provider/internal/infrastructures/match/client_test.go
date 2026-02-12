package match

import (
	"context"
	"errors"
	"testing"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/errors"
	matchv1 "github.com/ozzus/fan-avia/protos/gen/go/match/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type matchAdapterClientMock struct {
	resp *matchv1.GetMatchResponse
	err  error
}

func (m *matchAdapterClientMock) GetMatch(ctx context.Context, in *matchv1.GetMatchRequest, opts ...grpc.CallOption) (*matchv1.GetMatchResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func (m *matchAdapterClientMock) GetUpcomingMatches(ctx context.Context, in *matchv1.GetUpcomingMatchesRequest, opts ...grpc.CallOption) (*matchv1.GetUpcomingMatchesResponse, error) {
	return &matchv1.GetUpcomingMatchesResponse{}, nil
}

func TestClient_GetMatch_MapsNotFound(t *testing.T) {
	c := NewClient(&matchAdapterClientMock{
		err: status.Error(codes.NotFound, "not found"),
	}, time.Second)

	_, err := c.GetMatch(context.Background(), 16114)
	if !errors.Is(err, derr.ErrMatchNotFound) {
		t.Fatalf("unexpected error: got %v want %v", err, derr.ErrMatchNotFound)
	}
}

func TestClient_GetMatch_MapsResponseAndUTC(t *testing.T) {
	kickoffMSK := time.Date(2026, 2, 27, 22, 30, 0, 0, time.FixedZone("MSK", 3*60*60))
	c := NewClient(&matchAdapterClientMock{
		resp: &matchv1.GetMatchResponse{
			Match: &matchv1.Match{
				MatchId:                16114,
				KickoffUtc:             timestamppb.New(kickoffMSK),
				DestinationAirportIata: "LED",
				TicketsLink:            "https://tickets.fc-zenit.ru/football/tickets/#zenit",
				ClubHomeId:             "3",
				ClubAwayId:             "444",
				City:                   "Saint Petersburg",
				Stadium:                "Gazprom Arena",
			},
		},
	}, time.Second)

	got, err := c.GetMatch(context.Background(), 16114)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.MatchID != 16114 {
		t.Fatalf("unexpected match id: %d", got.MatchID)
	}
	wantUTC := time.Date(2026, 2, 27, 19, 30, 0, 0, time.UTC)
	if !got.KickoffUTC.Equal(wantUTC) {
		t.Fatalf("unexpected kickoff UTC: got %s want %s", got.KickoffUTC, wantUTC)
	}
}

func TestClient_GetMatch_IncompletePayload(t *testing.T) {
	c := NewClient(&matchAdapterClientMock{
		resp: &matchv1.GetMatchResponse{
			Match: &matchv1.Match{
				MatchId: 16114,
			},
		},
	}, time.Second)

	_, err := c.GetMatch(context.Background(), 16114)
	if err == nil {
		t.Fatal("expected incomplete payload error")
	}
}
