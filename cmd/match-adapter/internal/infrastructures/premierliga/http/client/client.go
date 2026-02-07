package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return dto.GetFullDataMatchResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		dtoResp, err := c.getFullDataMatchOnce(ctx, payload)
		if err == nil {
			return dtoResp, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, derr.ErrMatchNotFound) {
			return dto.GetFullDataMatchResponse{}, err
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
			return dto.GetFullDataMatchResponse{}, ctx.Err()
		case <-timer.C:
		}
	}

	if lastErr == nil {
		lastErr = derr.ErrSourceUnavailable
	}

	return dto.GetFullDataMatchResponse{}, lastErr
}

func (c *Client) getFullDataMatchOnce(ctx context.Context, payload []byte) (dto.GetFullDataMatchResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(payload))
	if err != nil {
		return dto.GetFullDataMatchResponse{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return dto.GetFullDataMatchResponse{}, err
		}
		return dto.GetFullDataMatchResponse{}, fmt.Errorf("%w: do request: %v", derr.ErrSourceUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusNotFound {
			return dto.GetFullDataMatchResponse{}, derr.ErrMatchNotFound
		}
		if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests {
			return dto.GetFullDataMatchResponse{}, fmt.Errorf("%w: unexpected status: %s", derr.ErrSourceUnavailable, resp.Status)
		}
		return dto.GetFullDataMatchResponse{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var dtoResp dto.GetFullDataMatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&dtoResp); err != nil {
		return dto.GetFullDataMatchResponse{}, fmt.Errorf("decode response: %w", err)
	}

	return dtoResp, nil
}
