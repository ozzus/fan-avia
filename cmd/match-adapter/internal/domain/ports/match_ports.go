package ports

import (
	"context"
	"time"

	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
)

type MatchRepository interface {
	GetByID(ctx context.Context, id models.MatchID) (models.Match, error)
	GetUpcoming(ctx context.Context, limit int, clubID string) ([]models.Match, error)
	GetClubs(ctx context.Context) ([]models.Club, error)
	Upsert(ctx context.Context, match models.Match) error
}

type MatchCache interface {
	GetByID(ctx context.Context, id models.MatchID) (models.Match, error)
	Set(ctx context.Context, match models.Match, ttl time.Duration) error
}

type MatchSource interface {
	FetchByID(ctx context.Context, id models.MatchID) (models.Match, error)
	FetchUpcomingIDs(ctx context.Context, from time.Time, to time.Time, limit int) ([]models.MatchID, error)
}

type CityIATAResolver interface {
	ResolveDestinationIATA(ctx context.Context, city string) (string, error)
}
