package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ozzus/fan-avia/cmd/api-gateway/internal/clients/match"
	"go.uber.org/zap"
)

const (
	defaultUpcomingLimit     = 12
	defaultClubUpcomingLimit = 100
	maxUpcomingLimit         = 100
)

type MatchHandler struct {
	log     *zap.Logger
	client  *match.Client
	timeout time.Duration
}

type matchLoadError struct {
	MatchID int64  `json:"match_id"`
	Error   string `json:"error"`
}

func NewMatchHandler(log *zap.Logger, client *match.Client, timeout time.Duration) *MatchHandler {
	return &MatchHandler{log: log, client: client, timeout: timeout}
}

func (h *MatchHandler) GetMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	matchID, parseErr := parseMatchIDFromPath(r.URL.Path)
	if parseErr != "" {
		writeError(w, http.StatusBadRequest, parseErr)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.GetMatch(ctx, matchID)
	if err != nil {
		h.log.Error("get match failed",
			zap.Error(err),
			zap.Int64("match_id", matchID),
		)
		writeError(w, http.StatusBadGateway, "match adapter error")
		return
	}

	clubsResp, err := h.client.GetClubs(ctx)
	if err != nil {
		h.log.Error("get clubs failed", zap.Error(err))
		writeError(w, http.StatusBadGateway, "match adapter error")
		return
	}

	writeJSON(w, http.StatusOK, mapMatch(resp.GetMatch(), buildClubIndex(clubsResp.GetClubs())))
}

func (h *MatchHandler) GetMatches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ids, ok := parseMatchIDs(r.URL.Query().Get("ids"))
	if !ok {
		writeError(w, http.StatusBadRequest, "ids query is required, example: /v1/matches?ids=16114,16115")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	clubsResp, err := h.client.GetClubs(ctx)
	if err != nil {
		h.log.Error("get clubs failed", zap.Error(err))
		writeError(w, http.StatusBadGateway, "match adapter error")
		return
	}
	clubIndex := buildClubIndex(clubsResp.GetClubs())

	matches := make([]matchResponse, 0, len(ids))
	errors := make([]matchLoadError, 0)
	for _, id := range ids {
		resp, err := h.client.GetMatch(ctx, id)
		if err != nil {
			h.log.Error("get match failed in list",
				zap.Error(err),
				zap.Int64("match_id", id),
			)
			errors = append(errors, matchLoadError{
				MatchID: id,
				Error:   "match adapter error",
			})
			continue
		}
		matches = append(matches, mapMatch(resp.GetMatch(), clubIndex))
	}

	if len(matches) == 0 {
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"error":  "match adapter error",
			"errors": errors,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"matches": matches,
		"errors":  errors,
	})
}

func (h *MatchHandler) GetUpcomingMatches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	clubID, _, clubErr := parsePositiveIntQuery(r, "club_id")
	if clubErr != "" {
		writeError(w, http.StatusBadRequest, "club_id must be a positive integer")
		return
	}

	limit := int32(defaultUpcomingLimit)
	if clubID != "" {
		limit = defaultClubUpcomingLimit
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > maxUpcomingLimit {
			parsed = maxUpcomingLimit
		}
		limit = int32(parsed)
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	fetchLimit := limit
	if clubID != "" && fetchLimit < 100 {
		fetchLimit = 100
	}

	resp, err := h.client.GetUpcomingMatches(ctx, fetchLimit, clubID)
	if err != nil {
		h.log.Error("get upcoming matches failed", zap.Error(err), zap.Int32("limit", limit))
		writeError(w, http.StatusBadGateway, "match adapter error")
		return
	}

	clubsResp, err := h.client.GetClubs(ctx)
	if err != nil {
		h.log.Error("get clubs failed", zap.Error(err))
		writeError(w, http.StatusBadGateway, "match adapter error")
		return
	}
	clubIndex := buildClubIndex(clubsResp.GetClubs())

	matches := cutMatchesByLimit(filterMatchesByClubID(resp.GetMatches(), clubID), limit)
	result := make([]matchResponse, 0, len(matches))
	for _, m := range matches {
		result = append(result, mapMatch(m, clubIndex))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"matches": result,
		"errors":  []matchLoadError{},
	})
}

func parseMatchIDFromPath(path string) (int64, string) {
	const prefix = "/v1/matches/"
	if !strings.HasPrefix(path, prefix) {
		return 0, "invalid path, expected /v1/matches/{id}"
	}

	idPart := strings.TrimPrefix(path, prefix)
	idPart = strings.Trim(idPart, "/")
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, "invalid path, expected /v1/matches/{id}"
	}

	id, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || id <= 0 {
		return 0, "invalid match_id"
	}
	return id, ""
}

func parseMatchIDs(value string) ([]int64, bool) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, false
	}

	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || id <= 0 {
			return nil, false
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil, false
	}

	return ids, true
}
