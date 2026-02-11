package airfare

import (
	"context"
	"time"

	airfarev1 "github.com/ozzus/fan-avia/protos/gen/go/airfare/v1"
)

type Client struct {
	client  airfarev1.AirfareProviderServiceClient
	timeout time.Duration
}

func NewClient(client airfarev1.AirfareProviderServiceClient, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Client{
		client:  client,
		timeout: timeout,
	}
}

func (c *Client) GetAirfareByMatch(ctx context.Context, matchID int64, originIATA string) (*airfarev1.GetAirfareByMatchResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.GetAirfareByMatch(reqCtx, &airfarev1.GetAirfareByMatchRequest{
		MatchId:    matchID,
		OriginIata: originIATA,
	})
}
