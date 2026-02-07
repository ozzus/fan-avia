package model

import (
	"time"

	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
)

type TokenData struct {
	MatchID      models.MatchID
	RefreshToken string
	ExpiresAt    time.Time
	IssuedAt     time.Time
}
