package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/dto"
	"go.uber.org/zap"
)

type diagnosticRepoMock struct {
	match     models.Match
	updatedAt time.Time
	err       error
}

func (m *diagnosticRepoMock) GetByIDWithUpdatedAt(_ context.Context, _ models.MatchID) (models.Match, time.Time, error) {
	return m.match, m.updatedAt, m.err
}

type diagnosticSourceMock struct {
	resp dto.GetFullDataMatchResponse
	err  error
}

func (m *diagnosticSourceMock) GetFullDataMatch(_ context.Context, _ int64) (dto.GetFullDataMatchResponse, error) {
	return m.resp, m.err
}

func TestDiagnosticHandler_GetMatchSnapshotSuccess(t *testing.T) {
	kickoff := time.Date(2026, 2, 27, 19, 30, 0, 0, time.UTC)
	clubHome := int64(3)
	clubAway := int64(444)

	repo := &diagnosticRepoMock{
		match: models.Match{
			ID:              "16114",
			HomeTeam:        "3",
			AwayTeam:        "444",
			City:            "Saint Petersburg",
			Stadium:         "Gazprom Arena",
			KickoffUTC:      kickoff,
			DestinationIATA: "LED",
			TicketsLink:     "https://tickets.example",
		},
		updatedAt: kickoff.Add(-2 * time.Hour),
	}
	source := &diagnosticSourceMock{
		resp: dto.GetFullDataMatchResponse{
			ID:          16114,
			Date:        "2026-02-27T19:30:00Z",
			City:        "Saint Petersburg",
			Stadium:     "Gazprom Arena",
			TicketsLink: "https://tickets.example",
			ClubHome:    &clubHome,
			ClubAway:    &clubAway,
		},
	}

	h := NewDiagnosticHandler(zap.NewNop(), repo, source, 3*time.Second)
	req := httptest.NewRequest(http.MethodGet, "/debug/match?match_id=16114", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var body debugMatchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.MatchID != "16114" {
		t.Fatalf("unexpected match_id: %q", body.MatchID)
	}
	if body.SourceRaw == nil || body.SourceMapped == nil {
		t.Fatalf("expected source payload in response")
	}
	if body.DBMapped == nil || body.DBUpdatedUTC == "" {
		t.Fatalf("expected db payload in response")
	}
	if body.Comparison == nil {
		t.Fatalf("expected comparison in response")
	}
	if !body.Comparison.Equal {
		t.Fatalf("expected equal comparison, got diff: %+v", body.Comparison)
	}
}

func TestDiagnosticHandler_GetMatchSnapshotInvalidID(t *testing.T) {
	h := NewDiagnosticHandler(zap.NewNop(), &diagnosticRepoMock{}, &diagnosticSourceMock{}, 2*time.Second)
	req := httptest.NewRequest(http.MethodGet, "/debug/match?match_id=abc", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}
