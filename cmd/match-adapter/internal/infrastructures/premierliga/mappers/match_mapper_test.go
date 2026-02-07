package mappers

import (
	"testing"
	"time"

	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/dto"
)

func TestToDomainMatch_NormalizesCityAndClubIDs(t *testing.T) {
	homeID := int64(10)
	awayID := int64(20)
	resp := dto.GetFullDataMatchResponse{
		ID:          1,
		Date:        "2026-02-07T15:30:00Z",
		City:        "Москва",
		Stadium:     "Luzhniki",
		TicketsLink: "https://tickets.test/match/1",
		ClubHome:    &homeID,
		ClubAway:    &awayID,
	}

	match, err := ToDomainMatch(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if match.City != "Moscow" {
		t.Fatalf("expected normalized city Moscow, got %q", match.City)
	}
	if match.HomeTeam != "10" || match.AwayTeam != "20" {
		t.Fatalf("expected club ids 10/20, got %s/%s", match.HomeTeam, match.AwayTeam)
	}
	if !match.KickoffUTC.Equal(time.Date(2026, 2, 7, 15, 30, 0, 0, time.UTC)) {
		t.Fatalf("unexpected kickoff: %v", match.KickoffUTC)
	}
}

func TestToDomainMatch_InvalidDate(t *testing.T) {
	resp := dto.GetFullDataMatchResponse{
		ID:   1,
		Date: "invalid-date",
	}

	_, err := ToDomainMatch(resp)
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}
