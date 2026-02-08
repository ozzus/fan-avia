package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/dto"
)

type Client struct {
	baseURL     string
	httpClient  *http.Client
	maxAttempts int
	baseBackoff time.Duration
}

const (
	fullDataMatchPath = "/api/getFullDataMatch"
	clubsPath         = "/api/getClubs"
	historyGamesPath  = "/api/getHistoryGames"
)

func NewClient(baseURL string, httpClient *http.Client, maxAttempts int, baseBackoff time.Duration) *Client {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if baseBackoff <= 0 {
		baseBackoff = 100 * time.Millisecond
	}

	return &Client{
		baseURL:     baseURL,
		httpClient:  httpClient,
		maxAttempts: maxAttempts,
		baseBackoff: baseBackoff,
	}
}

func (c *Client) GetFullDataMatch(ctx context.Context, id int64) (dto.GetFullDataMatchResponse, error) {
	reqBody := dto.GetFullDataMatchRequest{ID: id}
	var resp dto.GetFullDataMatchResponse
	err := c.postWithRetry(ctx, fullDataMatchPath, reqBody, &resp, derr.ErrMatchNotFound)
	return resp, err
}

func (c *Client) GetClubs(ctx context.Context, tournament *int64) ([]dto.Club, error) {
	reqBody := dto.GetClubsRequest{Tournament: tournament}
	var resp []dto.Club
	err := c.postWithRetry(ctx, clubsPath, reqBody, &resp, nil)
	return resp, err
}

func (c *Client) GetHistoryGames(ctx context.Context, id int64) (dto.GetHistoryGamesResponse, error) {
	reqBody := dto.GetHistoryGamesRequest{ID: id}
	var resp dto.GetHistoryGamesResponse
	err := c.postWithRetry(ctx, historyGamesPath, reqBody, &resp, nil)
	return resp, err
}

func (c *Client) postWithRetry(ctx context.Context, endpointPath string, reqBody any, out any, notFoundErr error) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		err := c.postOnce(ctx, endpointPath, payload, out, notFoundErr)
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || (notFoundErr != nil && errors.Is(err, notFoundErr)) {
			return err
		}
		lastErr = err

		if attempt == c.maxAttempts || !errors.Is(err, derr.ErrSourceUnavailable) {
			break
		}

		backoff := c.baseBackoff * time.Duration(1<<(attempt-1))
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	if lastErr == nil {
		lastErr = derr.ErrSourceUnavailable
	}

	return lastErr
}

func (c *Client) postOnce(ctx context.Context, endpointPath string, payload []byte, out any, notFoundErr error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL(endpointPath), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("%w: do request: %v", derr.ErrSourceUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusNotFound {
			if notFoundErr != nil {
				return notFoundErr
			}
			return fmt.Errorf("%w: endpoint not found: %s", derr.ErrSourceUnavailable, c.endpointURL(endpointPath))
		}
		if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("%w: unexpected status: %s", derr.ErrSourceUnavailable, resp.Status)
		}
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func (c *Client) endpointURL(path string) string {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return c.baseURL
	}
	u.Path = path
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
