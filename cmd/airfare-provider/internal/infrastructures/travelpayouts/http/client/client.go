package travelpayouts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/ports"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/infrastructures/travelpayouts/dto"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/infrastructures/travelpayouts/mappers"
)

type Client struct {
	baseURL    string
	token      string
	currency   string
	limit      int
	httpClient *http.Client
}

func NewClient(baseURL, token, currency string, limit int, timeout time.Duration) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.travelpayouts.com"
	}
	if strings.TrimSpace(currency) == "" {
		currency = "rub"
	}
	if limit <= 0 {
		limit = 30
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      strings.TrimSpace(token),
		currency:   strings.ToLower(strings.TrimSpace(currency)),
		limit:      limit,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) GetPrices(ctx context.Context, search ports.FareSearch) ([]int64, error) {
	if strings.TrimSpace(c.token) == "" {
		return nil, fmt.Errorf("travelpayouts token is empty")
	}

	reqURL, err := c.buildURL(search)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("travelpayouts request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("travelpayouts status: %s", resp.Status)
	}

	var payload dto.PriceForDatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode travelpayouts response: %w", err)
	}

	return mappers.ExtractPrices(payload.Data, search), nil
}

func (c *Client) buildURL(search ports.FareSearch) (string, error) {
	departDate := search.DateUTC.UTC().Format("2006-01-02")
	u, err := url.Parse(c.baseURL + "/aviasales/v3/prices_for_dates")
	if err != nil {
		return "", fmt.Errorf("parse travelpayouts base url: %w", err)
	}

	q := u.Query()
	q.Set("origin", strings.ToUpper(strings.TrimSpace(search.OriginIATA)))
	q.Set("destination", strings.ToUpper(strings.TrimSpace(search.DestinationIATA)))
	q.Set("departure_at", departDate)
	q.Set("currency", c.currency)
	q.Set("sorting", "price")
	q.Set("token", c.token)
	q.Set("limit", strconv.Itoa(c.limit))
	q.Set("one_way", "true")
	u.RawQuery = q.Encode()
	return u.String(), nil
}
