package ports

import (
	"context"

	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
)

type TokenRepository interface {
	SaveRefreshToken(ctx context.Context, matchID models.MatchID, refreshToken string) error
	GetMatchIDByRefreshToken(ctx context.Context, refreshToken string) (models.MatchID, error)
	DeleteRefreshToken(ctx context.Context, matchID models.MatchID, refreshToken string) error
	AddToBlacklist(ctx context.Context, token string) error
	IsInBlacklist(ctx context.Context, token string) (bool, error)
}
