package service

import (
	"context"
	"errors"
	"testing"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/ports"
	"go.uber.org/zap"
)

type testMatchReader struct {
	match ports.MatchSnapshot
	err   error
	calls int
}

func (m *testMatchReader) GetMatch(ctx context.Context, matchID int64) (ports.MatchSnapshot, error) {
	m.calls++
	if m.err != nil {
		return ports.MatchSnapshot{}, m.err
	}
	return m.match, nil
}

type testFareSource struct {
	err      error
	searches []ports.FareSearch
}

func (f *testFareSource) GetPrices(ctx context.Context, search ports.FareSearch) ([]int64, error) {
	f.searches = append(f.searches, search)
	if f.err != nil {
		return nil, f.err
	}
	return []int64{1111}, nil
}

type testCache struct {
	getResult ports.AirfareByMatch
	getErr    error
	setCalls  int
}

func (c *testCache) GetByMatchAndOrigin(ctx context.Context, matchID int64, originIATA string) (ports.AirfareByMatch, error) {
	if c.getErr != nil {
		return ports.AirfareByMatch{}, c.getErr
	}
	return c.getResult, nil
}

func (c *testCache) SetByMatchAndOrigin(ctx context.Context, matchID int64, originIATA string, payload ports.AirfareByMatch, ttl time.Duration) error {
	c.setCalls++
	return nil
}

func TestGetAirfareByMatch_UsesCacheHit(t *testing.T) {
	cache := &testCache{
		getResult: ports.AirfareByMatch{
			MatchID: 16114,
			Slots:   []ports.FareSlot{{Kind: ports.SlotOutDMinus2}},
		},
	}
	reader := &testMatchReader{}
	fares := &testFareSource{}
	svc := NewAirfareService(zap.NewNop(), reader, fares, cache, 10*time.Minute)

	got, err := svc.GetAirfareByMatch(context.Background(), 16114, "MOW")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.MatchID != 16114 {
		t.Fatalf("unexpected match id: %d", got.MatchID)
	}
	if reader.calls != 0 {
		t.Fatalf("match reader should not be called on cache hit, calls=%d", reader.calls)
	}
	if len(fares.searches) != 0 {
		t.Fatalf("fare source should not be called on cache hit, calls=%d", len(fares.searches))
	}
}

func TestGetAirfareByMatch_NormalizesKickoffToUTC(t *testing.T) {
	loc := time.FixedZone("MSK", 3*60*60)
	kickoff := time.Date(2026, 2, 27, 22, 30, 0, 0, loc) // equals 19:30 UTC

	cache := &testCache{getErr: derr.ErrAirfareNotFound}
	reader := &testMatchReader{
		match: ports.MatchSnapshot{
			MatchID:         16114,
			KickoffUTC:      kickoff,
			DestinationIATA: "LED",
			TicketsLink:     "link",
		},
	}
	fares := &testFareSource{}
	svc := NewAirfareService(zap.NewNop(), reader, fares, cache, 10*time.Minute)

	got, err := svc.GetAirfareByMatch(context.Background(), 16114, "MOW")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Slots) != 6 {
		t.Fatalf("unexpected slots count: got %d want 6", len(got.Slots))
	}
	wantDay := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)
	if !got.Slots[2].DateUTC.Equal(wantDay) {
		t.Fatalf("unexpected D0 date: got %s want %s", got.Slots[2].DateUTC, wantDay)
	}
	if len(fares.searches) != 6 {
		t.Fatalf("expected 6 fare searches, got %d", len(fares.searches))
	}
	arriveBy := fares.searches[2].ArriveNotLaterUTC
	if arriveBy == nil || !arriveBy.Equal(time.Date(2026, 2, 27, 17, 30, 0, 0, time.UTC)) {
		t.Fatalf("unexpected arrive-by constraint: %v", arriveBy)
	}
	departAfter := fares.searches[3].DepartNotBeforeUTC
	if departAfter == nil || !departAfter.Equal(time.Date(2026, 2, 27, 23, 30, 0, 0, time.UTC)) {
		t.Fatalf("unexpected depart-after constraint: %v", departAfter)
	}
}

func TestGetAirfareByMatch_AllSourceCallsFail(t *testing.T) {
	cache := &testCache{getErr: derr.ErrAirfareNotFound}
	reader := &testMatchReader{
		match: ports.MatchSnapshot{
			MatchID:         16114,
			KickoffUTC:      time.Date(2026, 2, 27, 19, 30, 0, 0, time.UTC),
			DestinationIATA: "LED",
		},
	}
	fares := &testFareSource{err: errors.New("source down")}
	svc := NewAirfareService(zap.NewNop(), reader, fares, cache, 10*time.Minute)

	_, err := svc.GetAirfareByMatch(context.Background(), 16114, "MOW")
	if !errors.Is(err, derr.ErrSourceTemporary) {
		t.Fatalf("unexpected error: got %v want %v", err, derr.ErrSourceTemporary)
	}
}

func TestGetAirfareByMatch_InvalidRoute_OriginEqualsDestination(t *testing.T) {
	cache := &testCache{getErr: derr.ErrAirfareNotFound}
	reader := &testMatchReader{
		match: ports.MatchSnapshot{
			MatchID:         16114,
			KickoffUTC:      time.Date(2026, 2, 27, 19, 30, 0, 0, time.UTC),
			DestinationIATA: "LED",
		},
	}
	fares := &testFareSource{}
	svc := NewAirfareService(zap.NewNop(), reader, fares, cache, 10*time.Minute)

	_, err := svc.GetAirfareByMatch(context.Background(), 16114, "LED")
	if !errors.Is(err, derr.ErrInvalidRoute) {
		t.Fatalf("unexpected error: got %v want %v", err, derr.ErrInvalidRoute)
	}
	if len(fares.searches) != 0 {
		t.Fatalf("fare source must not be called for invalid route, calls=%d", len(fares.searches))
	}
}
