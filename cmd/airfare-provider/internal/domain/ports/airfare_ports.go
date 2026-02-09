package ports

import (
	"context"
	"time"
)

type MatchSnapshot struct {
	MatchID         int64
	KickoffUTC      time.Time
	DestinationIATA string
	TicketsLink     string
	HomeClubID      string
	AwayClubID      string
	City            string
	Stadium         string
}

type MatchReader interface {
	GetMatch(ctx context.Context, matchID int64) (MatchSnapshot, error)
}

type SlotKind uint8

const (
	SlotUnknown SlotKind = iota
	SlotOutDMinus2
	SlotOutDMinus1
	SlotOutD0ArriveBy
	SlotRetD0DepartAfter
	SlotRetDPlus1
	SlotRetDPlus2
)

type Direction uint8

const (
	DirectionUnknown Direction = iota
	DirectionOut
	DirectionRet
)

type FareSlot struct {
	Kind      SlotKind
	Direction Direction
	DateUTC   time.Time
	Prices    []int64
}

type AirfareByMatch struct {
	MatchID     int64
	TicketsLink string
	Slots       []FareSlot
}

type AirfareCache interface {
	GetByMatchAndOrigin(ctx context.Context, matchID int64, originIATA string) (AirfareByMatch, error)
	SetByMatchAndOrigin(ctx context.Context, matchID int64, originIATA string, payload AirfareByMatch, ttl time.Duration) error
}

type FareSearch struct {
	OriginIATA         string
	DestinationIATA    string
	DateUTC            time.Time
	ArriveNotLaterUTC  *time.Time
	DepartNotBeforeUTC *time.Time
}

type FareSource interface {
	GetPrices(ctx context.Context, search FareSearch) ([]int64, error)
}
