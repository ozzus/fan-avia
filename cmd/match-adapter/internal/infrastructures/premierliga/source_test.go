package premierliga

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	plclient "github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/http/client"
)

func TestSource_FetchUpcomingIDs_FiltersSortsAndDeduplicates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/getTournaments":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 722, "name": "RPL", "dateFrom": "2025-07-01", "dateTo": "2026-05-31"},
				{"id": 719, "name": "RPL", "dateFrom": "2024-07-01", "dateTo": "2025-05-31"},
			})
		case "/api/getMatches":
			var req struct {
				Tournament int64 `json:"tournament"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req.Tournament != 722 {
				t.Fatalf("unexpected tournament %d", req.Tournament)
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"stage": 2939,
					"matches": []map[string]any{
						{"id": 2, "date": "2026-02-28UTC19:30:00"},
						{"id": 1, "date": "2026-02-27UTC19:30:00"},
						{"id": 1, "date": "2026-02-27UTC19:30:00"},
						{"id": 3, "date": "2026-04-01UTC19:30:00"},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := plclient.NewClient(srv.URL, srv.Client(), 1, 100*time.Millisecond)
	source := NewSource(client)

	from := time.Date(2026, 2, 26, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	ids, err := source.FetchUpcomingIDs(context.Background(), from, to, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}
	if ids[0] != "1" || ids[1] != "2" {
		t.Fatalf("expected ids [1 2], got %v", ids)
	}
}

func TestSource_FetchUpcomingIDs_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := plclient.NewClient(srv.URL, srv.Client(), 1, 100*time.Millisecond)
	source := NewSource(client)

	_, err := source.FetchUpcomingIDs(context.Background(), time.Now().UTC(), time.Now().UTC().Add(24*time.Hour), 10)
	if !errors.Is(err, derr.ErrSourceUnavailable) {
		t.Fatalf("expected ErrSourceUnavailable, got %v", err)
	}
}

func TestSource_FetchUpcomingIDs_UsesLatestTournamentFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/getTournaments":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 900, "name": "Old", "dateFrom": "2020-07-01", "dateTo": "2021-05-31"},
			})
		case "/api/getMatches":
			var req struct {
				Tournament int64 `json:"tournament"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req.Tournament != 900 {
				t.Fatalf("expected fallback tournament 900, got %d", req.Tournament)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"stage": 1, "matches": []map[string]any{{"id": 16114, "date": "2026-02-27UTC19:30:00"}}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := plclient.NewClient(srv.URL, srv.Client(), 1, 100*time.Millisecond)
	source := NewSource(client)

	from := time.Date(2026, 2, 26, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	ids, err := source.FetchUpcomingIDs(context.Background(), from, to, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(ids) != 1 || ids[0] != "16114" {
		t.Fatalf("unexpected ids: %v", ids)
	}
}
