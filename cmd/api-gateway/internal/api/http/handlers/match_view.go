package handlers

import (
	"strconv"
	"time"

	matchv1 "github.com/ozzus/fan-avia/protos/gen/go/match/v1"
)

var moscowLocation = mustLoadLocation("Europe/Moscow")

type clubView struct {
	ClubID      string `json:"club_id"`
	NameRU      string `json:"name_ru"`
	NameEN      string `json:"name_en,omitempty"`
	Logo        string `json:"logo,omitempty"`
	City        string `json:"city,omitempty"`
	AirportIATA string `json:"airport_iata,omitempty"`
}

type matchResponse struct {
	MatchID                string    `json:"match_id"`
	KickoffUTC             string    `json:"kickoff_utc,omitempty"`
	KickoffLocal           string    `json:"kickoff_local,omitempty"`
	City                   string    `json:"city,omitempty"`
	Stadium                string    `json:"stadium,omitempty"`
	DestinationAirportIATA string    `json:"destination_airport_iata,omitempty"`
	ClubHomeID             string    `json:"club_home_id,omitempty"`
	ClubAwayID             string    `json:"club_away_id,omitempty"`
	HomeClub               *clubView `json:"home_club,omitempty"`
	AwayClub               *clubView `json:"away_club,omitempty"`
	TicketsLink            string    `json:"tickets_link,omitempty"`
}

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("MSK", 3*60*60)
	}
	return loc
}

func buildClubIndex(clubs []*matchv1.Club) map[string]*clubView {
	index := make(map[string]*clubView, len(clubs))
	for _, c := range clubs {
		if c == nil {
			continue
		}
		index[c.GetClubId()] = &clubView{
			ClubID:      c.GetClubId(),
			NameRU:      c.GetNameRu(),
			NameEN:      c.GetNameEn(),
			Logo:        c.GetLogo(),
			City:        c.GetCity(),
			AirportIATA: c.GetAirportIata(),
		}
	}
	return index
}

func mapMatch(in *matchv1.Match, clubs map[string]*clubView) matchResponse {
	if in == nil {
		return matchResponse{}
	}

	out := matchResponse{
		MatchID:                strconv.FormatInt(in.GetMatchId(), 10),
		City:                   in.GetCity(),
		Stadium:                in.GetStadium(),
		DestinationAirportIATA: in.GetDestinationAirportIata(),
		ClubHomeID:             in.GetClubHomeId(),
		ClubAwayID:             in.GetClubAwayId(),
		HomeClub:               clubs[in.GetClubHomeId()],
		AwayClub:               clubs[in.GetClubAwayId()],
		TicketsLink:            in.GetTicketsLink(),
	}
	if in.GetKickoffUtc() != nil {
		kickoff := in.GetKickoffUtc().AsTime()
		out.KickoffUTC = kickoff.UTC().Format(time.RFC3339)
		out.KickoffLocal = kickoff.In(moscowLocation).Format(time.RFC3339)
	}

	return out
}
