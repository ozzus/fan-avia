package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
	"github.com/redis/go-redis/v9"
)

type MatchCache struct {
	redis *redis.Client
}

func NewMatchCache(redis *redis.Client) *MatchCache {
	return &MatchCache{redis: redis}
}

func (c *MatchCache) GetByID(ctx context.Context, id models.MatchID) (models.Match, error) {
	key := fmt.Sprintf("match:%s", id)
	data, err := c.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return models.Match{}, derr.ErrMatchNotFound
		}
		return models.Match{}, fmt.Errorf("redis get match by id: %w", err)
	}

	var match models.Match
	if err := json.Unmarshal([]byte(data), &match); err != nil {
		return models.Match{}, fmt.Errorf("unmarshal cached match: %w", err)
	}

	return match, nil
}

func (c *MatchCache) Set(ctx context.Context, match models.Match, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}

	normalized := match
	normalized.KickoffUTC = normalized.KickoffUTC.UTC()

	key := fmt.Sprintf("match:%s", match.ID)
	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal match for cache: %w", err)
	}

	if err := c.redis.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set match: %w", err)
	}

	return nil
}
