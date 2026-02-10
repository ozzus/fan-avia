package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/ports"
	"github.com/redis/go-redis/v9"
)

type AirfareCacheRepository struct {
	redis *redis.Client
}

func NewAirfareCacheRepository(redisClient *redis.Client) *AirfareCacheRepository {
	return &AirfareCacheRepository{redis: redisClient}
}

func (r *AirfareCacheRepository) GetByMatchAndOrigin(ctx context.Context, matchID int64, originIATA string) (ports.AirfareByMatch, error) {
	key := airfareKey(matchID, originIATA)
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ports.AirfareByMatch{}, derr.ErrAirfareNotFound
		}
		return ports.AirfareByMatch{}, fmt.Errorf("redis get airfare: %w", err)
	}

	var payload ports.AirfareByMatch
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return ports.AirfareByMatch{}, fmt.Errorf("unmarshal cached airfare: %w", err)
	}

	return payload, nil
}

func (r *AirfareCacheRepository) SetByMatchAndOrigin(ctx context.Context, matchID int64, originIATA string, payload ports.AirfareByMatch, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}

	key := airfareKey(matchID, originIATA)
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal airfare for cache: %w", err)
	}

	if err := r.redis.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set airfare: %w", err)
	}

	return nil
}

func airfareKey(matchID int64, originIATA string) string {
	return fmt.Sprintf("airfare:%d:%s", matchID, strings.ToUpper(strings.TrimSpace(originIATA)))
}
