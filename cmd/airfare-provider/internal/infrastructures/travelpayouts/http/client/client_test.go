package travelpayouts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/ports"
)

func TestGetPrices_SortsAndDeduplicates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"data":[
				{"price":3000,"departure_at":"2026-02-27T10:00:00Z"},
				{"price":1000,"departure_at":"2026-02-27T11:00:00Z"},
				{"price":1000,"departure_at":"2026-02-27T12:00:00Z"},
				{"price":0,"departure_at":"2026-02-27T13:00:00Z"},
				{"price":-1,"departure_at":"2026-02-27T14:00:00Z"}
			]
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token", "rub", 30, time.Second)
	got, err := c.GetPrices(context.Background(), ports.FareSearch{
		OriginIATA:      "MOW",
		DestinationIATA: "LED",
		DateUTC:         time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != 1000 || got[1] != 3000 {
		t.Fatalf("unexpected prices: %v", got)
	}
}

func TestGetPrices_ArriveNotLaterConstraint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"data":[
				{"price":2000,"departure_at":"2026-02-27T14:00:00Z","duration_to":120},
				{"price":1000,"departure_at":"2026-02-27T15:30:00Z","duration_to":120}
			]
		}`))
	}))
	defer srv.Close()

	limit := time.Date(2026, 2, 27, 16, 30, 0, 0, time.UTC)
	c := NewClient(srv.URL, "token", "rub", 30, time.Second)
	got, err := c.GetPrices(context.Background(), ports.FareSearch{
		OriginIATA:        "MOW",
		DestinationIATA:   "LED",
		DateUTC:           time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC),
		ArriveNotLaterUTC: &limit,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 2000 {
		t.Fatalf("unexpected prices after arrive_by filter: %v", got)
	}
}

func TestGetPrices_DepartNotBeforeConstraint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"data":[
				{"price":1500,"departure_at":"2026-02-27T12:00:00Z"},
				{"price":2500,"departure_at":"2026-02-27T18:00:00Z"},
				{"price":2200,"return_at":"2026-02-27T17:00:00Z"}
			]
		}`))
	}))
	defer srv.Close()

	limit := time.Date(2026, 2, 27, 15, 0, 0, 0, time.UTC)
	c := NewClient(srv.URL, "token", "rub", 30, time.Second)
	got, err := c.GetPrices(context.Background(), ports.FareSearch{
		OriginIATA:         "LED",
		DestinationIATA:    "MOW",
		DateUTC:            time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC),
		DepartNotBeforeUTC: &limit,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != 2200 || got[1] != 2500 {
		t.Fatalf("unexpected prices after depart_after filter: %v", got)
	}
}

func TestGetPrices_EmptyToken(t *testing.T) {
	c := NewClient("https://api.travelpayouts.com", "", "rub", 30, time.Second)
	_, err := c.GetPrices(context.Background(), ports.FareSearch{
		OriginIATA:      "MOW",
		DestinationIATA: "LED",
		DateUTC:         time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	if !strings.Contains(err.Error(), "token is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}
