package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ozzus/fan-avia/cmd/api-gateway/internal/clients/airfare"
	"github.com/ozzus/fan-avia/cmd/api-gateway/internal/clients/match"
	airfarev1 "github.com/ozzus/fan-avia/protos/gen/go/airfare/v1"
	"go.uber.org/zap"
)

const (
	defaultUpcomingWithAirfareLimit = 12
	maxUpcomingWithAirfareLimit     = 30
	defaultUpcomingWithAirfareTO    = 20 * time.Second
	maxConcurrentAirfareCalls       = 4
)

type CatalogHandler struct {
	log               *zap.Logger
	matchClient       *match.Client
	airfareClient     *airfare.Client
	timeout           time.Duration
	defaultOriginIATA string
}

type upcomingWithAirfareItem struct {
	Match        matchResponse `json:"match"`
	MinPrice     *int64        `json:"min_price,omitempty"`
	BestSlot     string        `json:"best_slot,omitempty"`
	BestDate     string        `json:"best_date,omitempty"`
	AirfareError string        `json:"airfare_error,omitempty"`
}

type airfareLoadError struct {
	MatchID string `json:"match_id"`
	Error   string `json:"error"`
}

type upcomingWithAirfareResponse struct {
	OriginIATA string                    `json:"origin_iata"`
	Items      []upcomingWithAirfareItem `json:"items"`
	Errors     []airfareLoadError        `json:"errors"`
}

func NewCatalogHandler(log *zap.Logger, matchClient *match.Client, airfareClient *airfare.Client, timeout time.Duration, defaultOriginIATA string) *CatalogHandler {
	if timeout <= 0 {
		timeout = defaultUpcomingWithAirfareTO
	}

	return &CatalogHandler{
		log:               log,
		matchClient:       matchClient,
		airfareClient:     airfareClient,
		timeout:           timeout,
		defaultOriginIATA: strings.ToUpper(strings.TrimSpace(defaultOriginIATA)),
	}
}

func (h *CatalogHandler) GetUpcomingWithAirfare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limit := int32(defaultUpcomingWithAirfareLimit)
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > maxUpcomingWithAirfareLimit {
			parsed = maxUpcomingWithAirfareLimit
		}
		limit = int32(parsed)
	}

	originIATA := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("origin_iata")))
	if originIATA == "" {
		originIATA = h.defaultOriginIATA
	}
	if originIATA == "" {
		writeError(w, http.StatusBadRequest, "origin_iata is required")
		return
	}
	if !isValidIATA(originIATA) {
		writeError(w, http.StatusBadRequest, "origin_iata must be 3 latin letters")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	upcomingResp, err := h.matchClient.GetUpcomingMatches(ctx, limit)
	if err != nil {
		h.log.Error("get upcoming matches failed", zap.Error(err), zap.Int32("limit", limit))
		writeError(w, http.StatusBadGateway, "match adapter error")
		return
	}

	items := make([]upcomingWithAirfareItem, 0, len(upcomingResp.GetMatches()))
	for _, m := range upcomingResp.GetMatches() {
		items = append(items, upcomingWithAirfareItem{Match: mapMatch(m)})
	}

	type airfareResult struct {
		index      int
		minPrice   *int64
		bestSlot   string
		bestDate   string
		errMessage string
	}

	sem := make(chan struct{}, maxConcurrentAirfareCalls)
	resultsCh := make(chan airfareResult, len(upcomingResp.GetMatches()))
	var wg sync.WaitGroup

	for i, m := range upcomingResp.GetMatches() {
		if strings.EqualFold(strings.TrimSpace(items[i].Match.DestinationAirportIATA), originIATA) {
			resultsCh <- airfareResult{
				index:      i,
				errMessage: "origin_iata and destination_iata must differ",
			}
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, matchID int64) {
			defer wg.Done()
			defer func() { <-sem }()

			airfareResp, err := h.airfareClient.GetAirfareByMatch(ctx, matchID, originIATA)
			if err != nil {
				resultsCh <- airfareResult{
					index:      idx,
					errMessage: mapGRPCError(err),
				}
				return
			}

			minPrice, bestSlot, bestDate := findBestFare(airfareResp.GetSlots())
			if minPrice == nil {
				resultsCh <- airfareResult{
					index:      idx,
					errMessage: "no airfare offers found",
				}
				return
			}

			resultsCh <- airfareResult{
				index:    idx,
				minPrice: minPrice,
				bestSlot: bestSlot,
				bestDate: bestDate,
			}
		}(i, m.GetMatchId())
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	resp := upcomingWithAirfareResponse{
		OriginIATA: originIATA,
		Items:      items,
		Errors:     make([]airfareLoadError, 0),
	}

	for r := range resultsCh {
		resp.Items[r.index].MinPrice = r.minPrice
		resp.Items[r.index].BestSlot = r.bestSlot
		resp.Items[r.index].BestDate = r.bestDate
		resp.Items[r.index].AirfareError = r.errMessage
		if r.errMessage != "" {
			resp.Errors = append(resp.Errors, airfareLoadError{
				MatchID: resp.Items[r.index].Match.MatchID,
				Error:   r.errMessage,
			})
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func findBestFare(slots []*airfarev1.FareSlot) (*int64, string, string) {
	var (
		minPrice int64
		hasPrice bool
		bestSlot string
		bestDate string
	)

	for _, slot := range slots {
		for _, price := range slot.GetPrices() {
			if !hasPrice || price < minPrice {
				hasPrice = true
				minPrice = price
				bestSlot = slot.GetSlot().String()
				bestDate = slot.GetDate()
			}
		}
	}

	if !hasPrice {
		return nil, "", ""
	}

	return &minPrice, bestSlot, bestDate
}
