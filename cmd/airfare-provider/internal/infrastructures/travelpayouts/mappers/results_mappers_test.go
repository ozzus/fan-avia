package mappers

import (
	"testing"
	"time"

	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/ports"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/infrastructures/travelpayouts/dto"
)

func TestExtractPrices_SortsAndDeduplicates(t *testing.T) {
	got := ExtractPrices([]dto.PriceForDateItem{
		{Price: 3000, DepartureAt: "2026-02-27T10:00:00Z"},
		{Price: 1000, DepartureAt: "2026-02-27T11:00:00Z"},
		{Price: 1000, DepartureAt: "2026-02-27T12:00:00Z"},
		{Price: 0, DepartureAt: "2026-02-27T13:00:00Z"},
		{Price: -1, DepartureAt: "2026-02-27T14:00:00Z"},
	}, ports.FareSearch{})

	if len(got) != 2 || got[0] != 1000 || got[1] != 3000 {
		t.Fatalf("unexpected prices: %v", got)
	}
}

func TestExtractPrices_ArriveNotLaterConstraint(t *testing.T) {
	limit := time.Date(2026, 2, 27, 16, 30, 0, 0, time.UTC)
	got := ExtractPrices([]dto.PriceForDateItem{
		{Price: 2000, DepartureAt: "2026-02-27T14:00:00Z", DurationTo: 120}, // arrival 16:00 pass
		{Price: 1000, DepartureAt: "2026-02-27T15:30:00Z", DurationTo: 120}, // arrival 17:30 fail
	}, ports.FareSearch{ArriveNotLaterUTC: &limit})

	if len(got) != 1 || got[0] != 2000 {
		t.Fatalf("unexpected prices with arrive_not_later: %v", got)
	}
}

func TestExtractPrices_DepartNotBeforeConstraint(t *testing.T) {
	limit := time.Date(2026, 2, 27, 15, 0, 0, 0, time.UTC)
	got := ExtractPrices([]dto.PriceForDateItem{
		{Price: 1500, DepartureAt: "2026-02-27T12:00:00Z"}, // fail
		{Price: 2500, DepartureAt: "2026-02-27T18:00:00Z"}, // pass
		{Price: 2200, ReturnAt: "2026-02-27T17:00:00Z"},    // pass via return_at fallback
	}, ports.FareSearch{DepartNotBeforeUTC: &limit})

	if len(got) != 2 || got[0] != 2200 || got[1] != 2500 {
		t.Fatalf("unexpected prices with depart_not_before: %v", got)
	}
}

func TestExtractPrices_ReturnsEmptySliceWhenNoMatches(t *testing.T) {
	limit := time.Date(2026, 2, 27, 15, 0, 0, 0, time.UTC)
	got := ExtractPrices([]dto.PriceForDateItem{
		{Price: 1000, DepartureAt: "2026-02-27T10:00:00Z"},
		{Price: 0, DepartureAt: "2026-02-27T20:00:00Z"},
	}, ports.FareSearch{DepartNotBeforeUTC: &limit})

	if got == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}
