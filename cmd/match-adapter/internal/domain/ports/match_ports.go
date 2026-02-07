package ports

import (
	"context"
	"time"

	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
)

type MatchRepository interface {
	GetByID(ctx context.Context, id models.MatchID) (models.Match, error)
	Upsert(ctx context.Context, match models.Match) error
}

type MatchCache interface {
	GetByID(ctx context.Context, id models.MatchID) (models.Match, error)
	Set(ctx context.Context, match models.Match, ttl time.Duration) error
}

type MatchSource interface {
	FetchByID(ctx context.Context, id models.MatchID) (models.Match, error)
}

type CityIATAResolver interface {
	ResolveDestinationIATA(ctx context.Context, city string) (string, error)
}
