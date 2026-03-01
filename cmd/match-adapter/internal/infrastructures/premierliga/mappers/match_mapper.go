package mappers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/dto"
)

func ToDomainMatch(resp dto.GetFullDataMatchResponse) (models.Match, error) {
	kickoff, err := parseKickoff(resp.Date)
	if err != nil {
		return models.Match{}, fmt.Errorf("parse kickoff datetime: %w", err)
	}

	return models.Match{
		ID:          models.MatchID(fmt.Sprintf("%d", resp.ID)),
		HomeTeam:    clubIDToString(resp.ClubHome),
		AwayTeam:    clubIDToString(resp.ClubAway),
		City:        normalizeCity(resp.City),
		Stadium:     resp.Stadium,
		TicketsLink: resp.TicketsLink,
		KickoffUTC:  kickoff.UTC(),
	}, nil
}

func clubIDToString(id *int64) string {
	if id == nil {
		return ""
	}
	return strconv.FormatInt(*id, 10)
}

func parseKickoff(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("unsupported datetime format: %q", value)
	}

	// RFC3339 values include explicit timezone and can be parsed directly.
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}

	// Premierliga returns local kickoff time in formats without timezone.
	loc := time.FixedZone("MSK", 3*60*60)
	if moscow, err := time.LoadLocation("Europe/Moscow"); err == nil {
		loc = moscow
	}

	localLayouts := []string{
		"2006-01-02UTC15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}

	for _, layout := range localLayouts {
		t, err := time.ParseInLocation(layout, value, loc)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported datetime format: %q", value)
}

func normalizeCity(city string) string {
	switch city {
	case "\u0421\u0430\u043d\u043a\u0442-\u041f\u0435\u0442\u0435\u0440\u0431\u0443\u0440\u0433":
		return "Saint Petersburg"
	case "\u041c\u043e\u0441\u043a\u0432\u0430":
		return "Moscow"
	case "\u041a\u0430\u043b\u0438\u043d\u0438\u043d\u0433\u0440\u0430\u0434":
		return "Kaliningrad"
	default:
		return city
	}
}
