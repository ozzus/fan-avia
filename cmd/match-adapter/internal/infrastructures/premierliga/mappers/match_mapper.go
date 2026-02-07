package mappers

import (
	"fmt"
	"strconv"
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
	layouts := []string{
		time.RFC3339,
		"2006-01-02UTC15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported datetime format: %q", value)
}

func normalizeCity(city string) string {
	switch city {
	case "Санкт-Петербург":
		return "Saint Petersburg"
	case "Москва":
		return "Moscow"
	case "Калининград":
		return "Kaliningrad"
	default:
		return city
	}
}
