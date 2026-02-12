package premierliga

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/dto"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/http/client"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/mappers"
)

type Source struct {
	client *client.Client
}

func NewSource(client *client.Client) *Source {
	return &Source{
		client: client,
	}
}

func (s *Source) FetchByID(ctx context.Context, id models.MatchID) (models.Match, error) {
	intID, err := strconv.ParseInt(string(id), 10, 64)
	if err != nil {
		return models.Match{}, fmt.Errorf("parse match id %q: %w", id, err)
	}

	resp, err := s.client.GetFullDataMatch(ctx, intID)
	if err != nil {
		if errors.Is(err, derr.ErrMatchNotFound) {
			return models.Match{}, derr.ErrMatchNotFound
		}
		if errors.Is(err, derr.ErrSourceUnavailable) {
			return models.Match{}, fmt.Errorf("get full data match: %w", derr.ErrSourceUnavailable)
		}
		return models.Match{}, fmt.Errorf("get full data match: %w", err)
	}

	match, err := mappers.ToDomainMatch(resp)
	if err != nil {
		return models.Match{}, err
	}

	return match, nil
}

func (s *Source) FetchUpcomingIDs(ctx context.Context, from time.Time, to time.Time, limit int) ([]models.MatchID, error) {
	tournaments, err := s.client.GetTournaments(ctx, dto.GetTournamentsRequest{Type: 1})
	if err != nil {
		if errors.Is(err, derr.ErrSourceUnavailable) {
			return nil, fmt.Errorf("get tournaments: %w", derr.ErrSourceUnavailable)
		}
		return nil, fmt.Errorf("get tournaments: %w", err)
	}

	if limit <= 0 {
		limit = 100
	}

	fromUTC := from.UTC()
	toUTC := to.UTC()

	type eventCandidate struct {
		id      models.MatchID
		kickoff time.Time
	}

	selected := selectTournamentsForRange(tournaments, fromUTC, toUTC)
	candidates := make([]eventCandidate, 0, limit)
	seen := make(map[models.MatchID]struct{}, limit)
	var lastErr error
	var loaded bool

	for _, tournament := range selected {
		stageItems, err := s.client.GetMatches(ctx, dto.GetMatchesRequest{Tournament: tournament.ID})
		if err != nil {
			if errors.Is(err, derr.ErrSourceUnavailable) {
				return nil, fmt.Errorf("get matches for tournament %d: %w", tournament.ID, derr.ErrSourceUnavailable)
			}
			lastErr = err
			continue
		}

		loaded = true
		for _, stage := range stageItems {
			for _, match := range stage.Matches {
				if match.ID <= 0 {
					continue
				}

				kickoff, err := parseKickoff(match.Date)
				if err != nil {
					continue
				}
				kickoffUTC := kickoff.UTC()
				if kickoffUTC.Before(fromUTC) || kickoffUTC.After(toUTC) {
					continue
				}

				id := models.MatchID(strconv.FormatInt(match.ID, 10))
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				candidates = append(candidates, eventCandidate{
					id:      id,
					kickoff: kickoffUTC,
				})
			}
		}
	}

	if !loaded && lastErr != nil {
		return nil, fmt.Errorf("get matches: %w", lastErr)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].kickoff.Before(candidates[j].kickoff)
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	ids := make([]models.MatchID, 0, len(candidates))
	for _, c := range candidates {
		ids = append(ids, c.id)
	}

	return ids, nil
}

func selectTournamentsForRange(tournaments []dto.Tournament, from, to time.Time) []dto.Tournament {
	if len(tournaments) == 0 {
		return nil
	}

	sort.Slice(tournaments, func(i, j int) bool {
		return tournaments[i].ID > tournaments[j].ID
	})

	selected := make([]dto.Tournament, 0, 3)
	for _, t := range tournaments {
		if t.ID <= 0 {
			continue
		}

		if tournamentOverlapsRange(t, from, to) {
			selected = append(selected, t)
			if len(selected) >= 3 {
				break
			}
		}
	}

	if len(selected) > 0 {
		return selected
	}

	for _, t := range tournaments {
		if t.ID <= 0 {
			continue
		}
		selected = append(selected, t)
		if len(selected) >= 2 {
			break
		}
	}

	return selected
}

func tournamentOverlapsRange(t dto.Tournament, from, to time.Time) bool {
	start, errStart := parseTournamentDay(t.DateFrom)
	end, errEnd := parseTournamentDay(t.DateTo)
	if errStart != nil || errEnd != nil {
		return false
	}

	end = end.Add(24*time.Hour - time.Nanosecond)
	if end.Before(from) {
		return false
	}
	if start.After(to) {
		return false
	}
	return true
}

func parseTournamentDay(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty tournament day")
	}
	return time.Parse("2006-01-02", value)
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
