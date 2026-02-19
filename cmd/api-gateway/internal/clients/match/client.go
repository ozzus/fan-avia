package match

import (
	"context"
	"strings"
	"time"

	matchv1 "github.com/ozzus/fan-avia/protos/gen/go/match/v1"
)

type Client struct {
	client  matchv1.MatchAdapterServiceClient
	timeout time.Duration
}

func NewClient(client matchv1.MatchAdapterServiceClient, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Client{
		client:  client,
		timeout: timeout,
	}
}

func (c *Client) GetMatch(ctx context.Context, matchID int64) (*matchv1.GetMatchResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.GetMatch(reqCtx, &matchv1.GetMatchRequest{MatchId: matchID})
}

func (c *Client) GetUpcomingMatches(ctx context.Context, limit int32, clubID string) (*matchv1.GetUpcomingMatchesResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.GetUpcomingMatches(reqCtx, &matchv1.GetUpcomingMatchesRequest{
		Limit:  limit,
		ClubId: strings.TrimSpace(clubID),
	})
}
