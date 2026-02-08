package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
)

func TestGetFullDataMatch_404OnFullDataEndpointMapsToNotFound(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	c := NewClient(srv.URL+"/api/getFullDataMatch", srv.Client(), 1, 100*time.Millisecond)
	_, err := c.GetFullDataMatch(context.Background(), 16114)
	if !errors.Is(err, derr.ErrMatchNotFound) {
		t.Fatalf("expected ErrMatchNotFound, got %v", err)
	}
}

func TestGetFullDataMatch_404OnWrongEndpointMapsToUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/api/getFullDataMatch-broken", srv.Client(), 1, 100*time.Millisecond)
	_, err := c.GetFullDataMatch(context.Background(), 16114)
	if !errors.Is(err, derr.ErrSourceUnavailable) {
		t.Fatalf("expected ErrSourceUnavailable, got %v", err)
	}
}

func TestGetClubs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/getClubs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := req["tournament"]; !ok {
			t.Fatal("expected tournament field in request")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":3,"name":"Зенит","nameShort":"ЗЕН","city":"Санкт-Петербург"}]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/api/getFullDataMatch", srv.Client(), 1, 100*time.Millisecond)
	clubs, err := c.GetClubs(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(clubs) != 1 {
		t.Fatalf("expected one club, got %d", len(clubs))
	}
	if clubs[0].ID != 3 || clubs[0].Name != "Зенит" {
		t.Fatalf("unexpected clubs payload: %+v", clubs[0])
	}
}

func TestGetHistoryGames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/getHistoryGames" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(payload), `"id":16114`) {
			t.Fatalf("unexpected payload: %s", string(payload))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"historyMatches":{"matches":3,"win":2,"draw":1,"loss":0},"lastMatches":[{"id":16033,"tournament":722,"stage":"Тур 8","clubH":444,"clubA":3,"goalH":0,"goalA":0,"date":"2025-09-14UTC17:00:00","videoReviewLink":null,"countVideos":1}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/api/getFullDataMatch", srv.Client(), 1, 100*time.Millisecond)
	history, err := c.GetHistoryGames(context.Background(), 16114)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if history.HistoryMatches.Matches != 3 || len(history.LastMatches) != 1 {
		t.Fatalf("unexpected history response: %+v", history)
	}
}
