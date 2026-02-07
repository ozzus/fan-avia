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
	match models.Match
	err   error
	calls int
}

func (m *sourceMock) FetchByID(_ context.Context, _ models.MatchID) (models.Match, error) {
	m.calls++
	return m.match, m.err
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
	getMatch    models.Match
	getErr      error
	upsertErr   error
	getCalls    int
	upsertCalls int
}

func (m *repoMock) GetByID(_ context.Context, _ models.MatchID) (models.Match, error) {
	m.getCalls++
	return m.getMatch, m.getErr
}

func (m *repoMock) Upsert(_ context.Context, _ models.Match) error {
	m.upsertCalls++
	return m.upsertErr
}

type cacheMock struct {
	getMatch models.Match
	getErr   error
	setErr   error
	getCalls int
	setCalls int
	lastTTL  time.Duration
}

func (m *cacheMock) GetByID(_ context.Context, _ models.MatchID) (models.Match, error) {
	m.getCalls++
	return m.getMatch, m.getErr
}

func (m *cacheMock) Set(_ context.Context, _ models.Match, ttl time.Duration) error {
	m.setCalls++
	m.lastTTL = ttl
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
