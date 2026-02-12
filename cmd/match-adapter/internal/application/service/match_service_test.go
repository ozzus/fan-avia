package service

import (
	"context"
	"errors"
	"testing"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
	"go.uber.org/zap"
)

type sourceMock struct {
	match         models.Match
	err           error
	calls         int
	matchByID     map[models.MatchID]models.Match
	errByID       map[models.MatchID]error
	upcomingIDs   []models.MatchID
	upcomingErr   error
	upcomingCalls int
}

func (m *sourceMock) FetchByID(_ context.Context, id models.MatchID) (models.Match, error) {
	m.calls++
	if err, ok := m.errByID[id]; ok {
		return models.Match{}, err
	}
	if match, ok := m.matchByID[id]; ok {
		return match, nil
	}
	return m.match, m.err
}

func (m *sourceMock) FetchUpcomingIDs(_ context.Context, _ time.Time, _ time.Time, _ int) ([]models.MatchID, error) {
	m.upcomingCalls++
	return m.upcomingIDs, m.upcomingErr
}

type resolverMock struct {
	iata  string
	err   error
	calls int
}

func (m *resolverMock) ResolveDestinationIATA(_ context.Context, _ string) (string, error) {
	m.calls++
	return m.iata, m.err
}

type repoMock struct {
	getMatch      models.Match
	getErr        error
	upcoming      []models.Match
	upcomingErr   error
	upsertErr     error
	upserted      []models.Match
	getCalls      int
	upcomingCalls int
	upsertCalls   int
}

func (m *repoMock) GetByID(_ context.Context, _ models.MatchID) (models.Match, error) {
	m.getCalls++
	return m.getMatch, m.getErr
}

func (m *repoMock) Upsert(_ context.Context, match models.Match) error {
	m.upsertCalls++
	m.upserted = append(m.upserted, match)
	return m.upsertErr
}

func (m *repoMock) GetUpcoming(_ context.Context, _ int) ([]models.Match, error) {
	m.upcomingCalls++
	return m.upcoming, m.upcomingErr
}

type cacheMock struct {
	getMatch models.Match
	getErr   error
	setErr   error
	setItems []models.Match
	getCalls int
	setCalls int
	lastTTL  time.Duration
}

func (m *cacheMock) GetByID(_ context.Context, _ models.MatchID) (models.Match, error) {
	m.getCalls++
	return m.getMatch, m.getErr
}

func (m *cacheMock) Set(_ context.Context, match models.Match, ttl time.Duration) error {
	m.setCalls++
	m.lastTTL = ttl
	m.setItems = append(m.setItems, match)
	return m.setErr
}

func TestGetMatch_CacheHit(t *testing.T) {
	cached := models.Match{ID: "100", City: "Moscow"}
	cache := &cacheMock{getMatch: cached}
	repo := &repoMock{}
	source := &sourceMock{}
	resolver := &resolverMock{}

	svc := NewMatchService(zap.NewNop(), source, resolver, repo, cache, 30*time.Minute)
	got, err := svc.GetMatch(context.Background(), "100")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.ID != cached.ID {
		t.Fatalf("expected cached match id %s, got %s", cached.ID, got.ID)
	}
	if repo.getCalls != 0 {
		t.Fatalf("expected repo not called, got %d calls", repo.getCalls)
	}
	if source.calls != 0 {
		t.Fatalf("expected source not called, got %d calls", source.calls)
	}
}

func TestGetMatch_CacheMissRepoHit(t *testing.T) {
	dbMatch := models.Match{ID: "200", City: "Kazan", DestinationIATA: "KZN"}
	cache := &cacheMock{getErr: derr.ErrMatchNotFound}
	repo := &repoMock{getMatch: dbMatch}
	source := &sourceMock{}
	resolver := &resolverMock{}

	ttl := 15 * time.Minute
	svc := NewMatchService(zap.NewNop(), source, resolver, repo, cache, ttl)
	got, err := svc.GetMatch(context.Background(), "200")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.ID != dbMatch.ID {
		t.Fatalf("expected db match id %s, got %s", dbMatch.ID, got.ID)
	}
	if cache.setCalls != 1 {
		t.Fatalf("expected cache set call, got %d", cache.setCalls)
	}
	if cache.lastTTL != ttl {
		t.Fatalf("expected ttl %v, got %v", ttl, cache.lastTTL)
	}
	if source.calls != 0 {
		t.Fatalf("expected source not called, got %d", source.calls)
	}
}

func TestGetMatch_SourceAndResolverFail(t *testing.T) {
	cache := &cacheMock{getErr: derr.ErrMatchNotFound}
	repo := &repoMock{getErr: derr.ErrMatchNotFound}
	source := &sourceMock{match: models.Match{ID: "300", City: "Unknown"}}
	resolver := &resolverMock{err: errors.New("resolve fail")}

	svc := NewMatchService(zap.NewNop(), source, resolver, repo, cache, 10*time.Minute)
	_, err := svc.GetMatch(context.Background(), "300")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver call, got %d", resolver.calls)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("expected no upsert when resolver fails, got %d", repo.upsertCalls)
	}
}

func TestGetMatch_SourceFail(t *testing.T) {
	cache := &cacheMock{getErr: derr.ErrMatchNotFound}
	repo := &repoMock{getErr: derr.ErrMatchNotFound}
	source := &sourceMock{err: derr.ErrSourceUnavailable}
	resolver := &resolverMock{}

	svc := NewMatchService(zap.NewNop(), source, resolver, repo, cache, 10*time.Minute)
	_, err := svc.GetMatch(context.Background(), "400")
	if !errors.Is(err, derr.ErrSourceUnavailable) {
		t.Fatalf("expected source unavailable error, got %v", err)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("expected no upsert when source fails, got %d", repo.upsertCalls)
	}
}

func TestGetUpcomingMatches_DefaultLimit(t *testing.T) {
	repo := &repoMock{
		upcoming: []models.Match{
			{ID: "1", City: "Moscow"},
			{ID: "2", City: "Kazan"},
		},
	}
	svc := NewMatchService(zap.NewNop(), &sourceMock{}, &resolverMock{}, repo, &cacheMock{}, 10*time.Minute)

	got, err := svc.GetUpcomingMatches(context.Background(), 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	if repo.upcomingCalls != 1 {
		t.Fatalf("expected 1 repo call, got %d", repo.upcomingCalls)
	}
}

func TestSyncUpcomingMatches_Success(t *testing.T) {
	kickoff := time.Date(2026, 2, 27, 19, 30, 0, 0, time.UTC)
	source := &sourceMock{
		upcomingIDs: []models.MatchID{"16114"},
		matchByID: map[models.MatchID]models.Match{
			"16114": {
				ID:         "16114",
				HomeTeam:   "3",
				AwayTeam:   "444",
				City:       "Saint Petersburg",
				Stadium:    "Gazprom Arena",
				KickoffUTC: kickoff,
			},
		},
	}
	resolver := &resolverMock{iata: "LED"}
	repo := &repoMock{}
	cache := &cacheMock{}
	svc := NewMatchService(zap.NewNop(), source, resolver, repo, cache, 30*time.Minute)

	count, err := svc.SyncUpcomingMatches(
		context.Background(),
		kickoff.Add(-24*time.Hour),
		kickoff.Add(24*time.Hour),
		10,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 1 {
		t.Fatalf("expected synced count 1, got %d", count)
	}
	if source.upcomingCalls != 1 {
		t.Fatalf("expected 1 upcoming source call, got %d", source.upcomingCalls)
	}
	if source.calls != 1 {
		t.Fatalf("expected 1 fetch-by-id call, got %d", source.calls)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver called once, got %d", resolver.calls)
	}
	if repo.upsertCalls != 1 {
		t.Fatalf("expected 1 upsert, got %d", repo.upsertCalls)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].DestinationIATA != "LED" {
		t.Fatalf("expected upserted match with destination LED, got %+v", repo.upserted)
	}
	if cache.setCalls != 1 {
		t.Fatalf("expected 1 cache set, got %d", cache.setCalls)
	}
}

func TestSyncUpcomingMatches_PartialFailure(t *testing.T) {
	kickoff := time.Date(2026, 2, 27, 19, 30, 0, 0, time.UTC)
	source := &sourceMock{
		upcomingIDs: []models.MatchID{"1", "2"},
		errByID: map[models.MatchID]error{
			"1": errors.New("fetch failed"),
		},
		matchByID: map[models.MatchID]models.Match{
			"2": {
				ID:              "2",
				City:            "Saint Petersburg",
				Stadium:         "Gazprom Arena",
				KickoffUTC:      kickoff,
				DestinationIATA: "LED",
			},
		},
	}
	resolver := &resolverMock{iata: "LED"}
	repo := &repoMock{}
	svc := NewMatchService(zap.NewNop(), source, resolver, repo, &cacheMock{}, 15*time.Minute)

	count, err := svc.SyncUpcomingMatches(
		context.Background(),
		kickoff.Add(-48*time.Hour),
		kickoff.Add(48*time.Hour),
		10,
	)
	if err != nil {
		t.Fatalf("expected no error with partial failure, got %v", err)
	}
	if count != 1 {
		t.Fatalf("expected synced count 1, got %d", count)
	}
	if repo.upsertCalls != 1 {
		t.Fatalf("expected one successful upsert, got %d", repo.upsertCalls)
	}
	if resolver.calls != 0 {
		t.Fatalf("expected resolver not called because destination already set, got %d", resolver.calls)
	}
}

func TestSyncUpcomingMatches_AllFailed(t *testing.T) {
	source := &sourceMock{
		upcomingIDs: []models.MatchID{"1"},
		errByID: map[models.MatchID]error{
			"1": errors.New("fetch failed"),
		},
	}
	repo := &repoMock{}
	svc := NewMatchService(zap.NewNop(), source, &resolverMock{}, repo, &cacheMock{}, 15*time.Minute)

	count, err := svc.SyncUpcomingMatches(context.Background(), time.Now(), time.Now().Add(24*time.Hour), 10)
	if err == nil {
		t.Fatal("expected error when all items fail")
	}
	if count != 0 {
		t.Fatalf("expected synced count 0, got %d", count)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("expected no upserts, got %d", repo.upsertCalls)
	}
}
