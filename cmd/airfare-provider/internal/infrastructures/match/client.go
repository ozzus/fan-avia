package match

import (
	"context"
	"errors"
	"fmt"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/ports"
	matchv1 "github.com/ozzus/fan-avia/protos/gen/go/match/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	client  matchv1.MatchAdapterServiceClient
	timeout time.Duration
}

func NewClient(client matchv1.MatchAdapterServiceClient, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	return &Client{
		client:  client,
		timeout: timeout,
	}
}

func (c *Client) GetMatch(ctx context.Context, matchID int64) (ports.MatchSnapshot, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetMatch(reqCtx, &matchv1.GetMatchRequest{MatchId: matchID})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ports.MatchSnapshot{}, err
		}
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				return ports.MatchSnapshot{}, derr.ErrMatchNotFound
			case codes.Unavailable, codes.DeadlineExceeded:
				return ports.MatchSnapshot{}, derr.ErrSourceTemporary
			}
		}
		return ports.MatchSnapshot{}, fmt.Errorf("get match from match-adapter: %w", err)
	}

	match := resp.GetMatch()
	if match == nil || match.GetKickoffUtc() == nil {
		return ports.MatchSnapshot{}, fmt.Errorf("match-adapter returned incomplete payload")
	}

	return ports.MatchSnapshot{
		MatchID:         match.GetMatchId(),
		KickoffUTC:      match.GetKickoffUtc().AsTime().UTC(),
		DestinationIATA: match.GetDestinationAirportIata(),
		TicketsLink:     match.GetTicketsLink(),
		HomeClubID:      match.GetClubHomeId(),
		AwayClubID:      match.GetClubAwayId(),
		City:            match.GetCity(),
		Stadium:         match.GetStadium(),
	}, nil
}
