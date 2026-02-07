package models

import "time"

type MatchID string

type Match struct {
	ID              MatchID
	HomeTeam        string
	AwayTeam        string
	City            string
	Stadium         string
	DestinationIATA string
	TicketsLink     string
	KickoffUTC      time.Time
}
