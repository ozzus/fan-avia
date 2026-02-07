package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/dto"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (c *Client) GetFullDataMatch(ctx context.Context, id int64) (dto.GetFullDataMatchResponse, error) {
	reqBody := dto.GetFullDataMatchRequest{ID: id}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return dto.GetFullDataMatchResponse{}, fmt.Errorf("marshal request: %w", err)
	}

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
