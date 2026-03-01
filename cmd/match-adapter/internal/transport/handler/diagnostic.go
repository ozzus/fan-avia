package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/dto"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/mappers"
	"go.uber.org/zap"
)

const defaultDiagnosticTimeout = 5 * time.Second

type dbMatchReader interface {
	GetByIDWithUpdatedAt(ctx context.Context, id models.MatchID) (models.Match, time.Time, error)
}

type sourceMatchReader interface {
	GetFullDataMatch(ctx context.Context, id int64) (dto.GetFullDataMatchResponse, error)
}

type DiagnosticHandler struct {
	log     *zap.Logger
	db      dbMatchReader
	source  sourceMatchReader
	timeout time.Duration
}

type debugMatch struct {
	MatchID         string `json:"match_id"`
	KickoffUTC      string `json:"kickoff_utc,omitempty"`
	City            string `json:"city,omitempty"`
	Stadium         string `json:"stadium,omitempty"`
	DestinationIATA string `json:"destination_iata,omitempty"`
	TicketsLink     string `json:"tickets_link,omitempty"`
	ClubHomeID      string `json:"club_home_id,omitempty"`
	ClubAwayID      string `json:"club_away_id,omitempty"`
}

type debugComparison struct {
	Equal          bool     `json:"equal"`
	DiffFields     []string `json:"diff_fields,omitempty"`
	SourceOnly     []string `json:"source_only,omitempty"`
	DatabaseOnly   []string `json:"database_only,omitempty"`
	HasSourceMatch bool     `json:"has_source_match"`
	HasDBMatch     bool     `json:"has_db_match"`
}

type debugMatchResponse struct {
	MatchID      string                        `json:"match_id"`
	CheckedAtUTC string                        `json:"checked_at_utc"`
	SourceRaw    *dto.GetFullDataMatchResponse `json:"source_raw,omitempty"`
	SourceMapped *debugMatch                   `json:"source_mapped,omitempty"`
	SourceError  string                        `json:"source_error,omitempty"`
	DBMapped     *debugMatch                   `json:"db_mapped,omitempty"`
	DBUpdatedUTC string                        `json:"db_updated_utc,omitempty"`
	DBError      string                        `json:"db_error,omitempty"`
	Comparison   *debugComparison              `json:"comparison,omitempty"`
}

func NewDiagnosticHandler(log *zap.Logger, db dbMatchReader, source sourceMatchReader, timeout time.Duration) http.Handler {
	if log == nil {
		log = zap.NewNop()
	}
	if timeout <= 0 {
		timeout = defaultDiagnosticTimeout
	}

	h := &DiagnosticHandler{
		log:     log,
		db:      db,
		source:  source,
		timeout: timeout,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/healthz", h.healthz)
	mux.HandleFunc("/debug/match", h.getMatchSnapshot)
	return mux
}

func (h *DiagnosticHandler) healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeDiagnosticError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	_, _ = w.Write([]byte("ok"))
}

func (h *DiagnosticHandler) getMatchSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeDiagnosticError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	idRaw := strings.TrimSpace(r.URL.Query().Get("match_id"))
	if idRaw == "" {
		writeDiagnosticError(w, http.StatusBadRequest, "match_id is required")
		return
	}

	matchID, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil || matchID <= 0 {
		writeDiagnosticError(w, http.StatusBadRequest, "match_id must be a positive integer")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	resp := debugMatchResponse{
		MatchID:      idRaw,
		CheckedAtUTC: time.Now().UTC().Format(time.RFC3339),
	}

	var sourceMatch *models.Match
	raw, err := h.source.GetFullDataMatch(ctx, matchID)
	if err != nil {
		resp.SourceError = err.Error()
	} else {
		resp.SourceRaw = &raw
		mapped, mapErr := mappers.ToDomainMatch(raw)
		if mapErr != nil {
			resp.SourceError = mapErr.Error()
		} else {
			sourceMatch = &mapped
			resp.SourceMapped = toDebugMatch(mapped)
		}
	}

	var dbMatch *models.Match
	match, updatedAt, err := h.db.GetByIDWithUpdatedAt(ctx, models.MatchID(idRaw))
	if err != nil {
		if errors.Is(err, derr.ErrMatchNotFound) {
			resp.DBError = "match not found in database"
		} else {
			resp.DBError = err.Error()
		}
	} else {
		dbMatch = &match
		resp.DBMapped = toDebugMatch(match)
		resp.DBUpdatedUTC = updatedAt.UTC().Format(time.RFC3339)
	}

	resp.Comparison = buildComparison(sourceMatch, dbMatch)
	writeDiagnosticJSON(w, http.StatusOK, resp)
}

func buildComparison(sourceMatch *models.Match, dbMatch *models.Match) *debugComparison {
	cmp := &debugComparison{
		HasSourceMatch: sourceMatch != nil,
		HasDBMatch:     dbMatch != nil,
	}

	if sourceMatch == nil || dbMatch == nil {
		cmp.Equal = false
		if sourceMatch != nil {
			cmp.SourceOnly = []string{"match"}
		}
		if dbMatch != nil {
			cmp.DatabaseOnly = []string{"match"}
		}
		return cmp
	}

	diff := make([]string, 0, 8)
	if sourceMatch.HomeTeam != dbMatch.HomeTeam {
		diff = append(diff, "club_home_id")
	}
	if sourceMatch.AwayTeam != dbMatch.AwayTeam {
		diff = append(diff, "club_away_id")
	}
	if sourceMatch.City != dbMatch.City {
		diff = append(diff, "city")
	}
	if sourceMatch.Stadium != dbMatch.Stadium {
		diff = append(diff, "stadium")
	}
	if sourceMatch.KickoffUTC.UTC() != dbMatch.KickoffUTC.UTC() {
		diff = append(diff, "kickoff_utc")
	}
	if sourceMatch.TicketsLink != dbMatch.TicketsLink {
		diff = append(diff, "tickets_link")
	}
	if sourceMatch.DestinationIATA != "" && dbMatch.DestinationIATA != "" && sourceMatch.DestinationIATA != dbMatch.DestinationIATA {
		diff = append(diff, "destination_iata")
	}

	cmp.Equal = len(diff) == 0
	cmp.DiffFields = diff
	return cmp
}

func toDebugMatch(m models.Match) *debugMatch {
	out := &debugMatch{
		MatchID:         string(m.ID),
		City:            m.City,
		Stadium:         m.Stadium,
		DestinationIATA: m.DestinationIATA,
		TicketsLink:     m.TicketsLink,
		ClubHomeID:      m.HomeTeam,
		ClubAwayID:      m.AwayTeam,
	}
	if !m.KickoffUTC.IsZero() {
		out.KickoffUTC = m.KickoffUTC.UTC().Format(time.RFC3339)
	}
	return out
}

func writeDiagnosticError(w http.ResponseWriter, statusCode int, message string) {
	writeDiagnosticJSON(w, statusCode, map[string]string{"error": message})
}

func writeDiagnosticJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}
