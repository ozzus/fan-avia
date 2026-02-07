package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/db/model"
	"github.com/redis/go-redis/v9"
)

type TokenRepository struct {
	redis      *redis.Client
	refreshTTL time.Duration
}

func NewTokenRepository(redis *redis.Client, refreshTTL time.Duration) *TokenRepository {
	return &TokenRepository{redis: redis, refreshTTL: refreshTTL}
}

func (r *TokenRepository) SaveRefreshToken(ctx context.Context, matchID models.MatchID, refreshToken string) error {
	tokenData := model.TokenData{
		MatchID:      matchID,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(r.refreshTTL),
	}
	key := fmt.Sprintf("refresh:%s:%s", matchID, refreshToken)
	data, err := json.Marshal(tokenData)
	if err != nil {
		return err
	}
	if err := r.redis.Set(ctx, key, data, r.refreshTTL).Err(); err != nil {
		return err
	}
	return r.storeReverseMapping(ctx, matchID, refreshToken)
}

func (r *TokenRepository) GetMatchIDByRefreshToken(ctx context.Context, refreshToken string) (models.MatchID, error) {
	key := fmt.Sprintf("refresh_token:%s", refreshToken)
	matchID, err := r.redis.Get(ctx, key).Result()
	if err == nil {
		return models.MatchID(matchID), nil
	}
	if !errors.Is(err, redis.Nil) {
		return "", err
	}
	pattern := fmt.Sprintf("refresh:*:%s", refreshToken)
	keys, findErr := r.redis.Keys(ctx, pattern).Result()
	if findErr != nil {
		return "", findErr
	}
	if len(keys) == 0 {
		return "", redis.Nil
	}
	parts := strings.Split(keys[0], ":")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid refresh token key format")
	}
	matchID = parts[1]
	_ = r.storeReverseMapping(ctx, models.MatchID(matchID), refreshToken)
	return models.MatchID(matchID), nil
}

func (r *TokenRepository) DeleteRefreshToken(ctx context.Context, matchID models.MatchID, refreshToken string) error {
	var tokenData model.TokenData
	if refreshToken == "" {
		pattern := fmt.Sprintf("refresh:%s:*", matchID)
		keys, err := r.redis.Keys(ctx, pattern).Result()
		if err != nil {
			return fmt.Errorf("failed to find tokens for match: %w", err)
		}
		if len(keys) == 0 {
			return nil
		}
		for _, key := range keys {
			parts := strings.Split(key, ":")
			if len(parts) == 3 {
				rt := parts[2]
				r.redis.Del(ctx, fmt.Sprintf("refresh_token:%s", rt))
			}
			if err := r.redis.Del(ctx, key).Err(); err != nil {
				return fmt.Errorf("failed to delete token %s: %w", key, err)
			}
		}
		return nil
	}
	key := fmt.Sprintf("refresh:%s:%s", matchID, refreshToken)
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	if err = json.Unmarshal([]byte(data), &tokenData); err != nil {
		return err
	}
	if tokenData.MatchID == matchID {
		if err := r.redis.Del(ctx, key).Err(); err != nil {
			return err
		}
		if err := r.redis.Del(ctx, fmt.Sprintf("refresh_token:%s", refreshToken)).Err(); err != nil {
			return err
		}
	} else {
		return errors.New("match id not match")
	}
	return nil
}

func (r *TokenRepository) AddToBlacklist(ctx context.Context, token string) error {
	key := fmt.Sprintf("blacklist:%s", token)
	if err := r.redis.Set(ctx, key, token, r.refreshTTL).Err(); err != nil {
		return err
	}
	return nil
}

func (r *TokenRepository) IsInBlacklist(ctx context.Context, token string) (bool, error) {
	key := fmt.Sprintf("blacklist:%s", token)
	exists, err := r.redis.Exists(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}
	return exists == 1, nil
}

func (r *TokenRepository) storeReverseMapping(ctx context.Context, matchID models.MatchID, refreshToken string) error {
	key := fmt.Sprintf("refresh_token:%s", refreshToken)
	return r.redis.Set(ctx, key, string(matchID), r.refreshTTL).Err()
}
