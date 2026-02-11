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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
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
	tracer := otel.Tracer("airfare-provider/travelpayouts-client")
	ctx, span := tracer.Start(ctx, "travelpayouts.client.GetPrices")
	defer span.End()
	span.SetAttributes(
		attribute.String("airfare.origin_iata", strings.ToUpper(strings.TrimSpace(search.OriginIATA))),
		attribute.String("airfare.destination_iata", strings.ToUpper(strings.TrimSpace(search.DestinationIATA))),
		attribute.String("airfare.date_utc", search.DateUTC.UTC().Format("2006-01-02")),
	)

	if strings.TrimSpace(c.token) == "" {
		err := fmt.Errorf("travelpayouts token is empty")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "empty token")
		return nil, err
	}

	reqURL, err := c.buildURL(search)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to build url")
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		wrapped := fmt.Errorf("build request: %w", err)
		span.RecordError(wrapped)
		span.SetStatus(otelcodes.Error, "failed to build request")
		return nil, wrapped
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		wrapped := fmt.Errorf("travelpayouts request: %w", err)
		span.RecordError(wrapped)
		span.SetStatus(otelcodes.Error, "request failed")
		return nil, wrapped
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("travelpayouts status: %s", resp.Status)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "non-2xx response")
		return nil, err
	}

	var payload dto.PriceForDatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		wrapped := fmt.Errorf("decode travelpayouts response: %w", err)
		span.RecordError(wrapped)
		span.SetStatus(otelcodes.Error, "decode failed")
		return nil, wrapped
	}

	prices := mappers.ExtractPrices(payload.Data, search)
	span.SetAttributes(attribute.Int("airfare.prices_count", len(prices)))
	span.SetStatus(otelcodes.Ok, "ok")
	return prices, nil
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
