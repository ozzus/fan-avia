package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/ozzus/fan-avia/cmd/match-adapter/grpcapp"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/application/service"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/config"
	matchdb "github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/db/postgres/repo"
	matchredis "github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/db/redis"
	matchtracing "github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/db/tracing"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga"
	plclient "github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/http/client"
	grpcapi "github.com/ozzus/fan-avia/cmd/match-adapter/internal/transport/grpc"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

func main() {
	_ = godotenv.Load(".env")

	cfg := config.MustLoad()
	log := setupLogger(cfg.Log.Level)
	defer func() {
		_ = log.Sync()
	}()

	tp, err := matchtracing.InitTracer("match-adapter", cfg.Jaeger)
	if err != nil {
		log.Fatal("failed to init tracer", zap.Error(err))
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			log.Warn("failed to shutdown tracer provider", zap.Error(err))
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("match-adapter starting", zap.String("grpc_addr", fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port)))

	repo, err := matchdb.New(ctx, cfg.DB.DatabaseURL())
	if err != nil {
		log.Fatal("failed to connect postgres", zap.Error(err))
	}
	defer repo.Close()

	httpClient := &http.Client{Timeout: cfg.Premierliga.Timeout}
	matchSource := premierliga.NewSource(plclient.NewClient(
		cfg.Premierliga.BaseURL,
		httpClient,
		cfg.Premierliga.RetryMaxAttempts,
		cfg.Premierliga.RetryBaseInterval,
	))
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Warn("failed to close redis client", zap.Error(err))
		}
	}()

	matchCache := matchredis.NewMatchCache(redisClient)
	matchService := service.NewMatchService(log, matchSource, repo, repo, matchCache, cfg.MatchCacheTTL)

	if cfg.MatchSync.Enabled {
		interval := cfg.MatchSync.Interval
		if interval <= 0 {
			interval = 15 * time.Minute
		}
		horizon := cfg.MatchSync.Horizon
		if horizon <= 0 {
			horizon = 30 * 24 * time.Hour
		}
		requestTimeout := cfg.MatchSync.RequestTimeout
		if requestTimeout <= 0 {
			requestTimeout = 30 * time.Second
		}

		runSync := func(trigger string) {
			now := time.Now().UTC()
			syncCtx, cancel := context.WithTimeout(ctx, requestTimeout)
			defer cancel()

			saved, err := matchService.SyncUpcomingMatches(syncCtx, now, now.Add(horizon), cfg.MatchSync.Limit)
			if err != nil {
				log.Warn(
					"upcoming matches sync failed",
					zap.String("trigger", trigger),
					zap.Error(err),
				)
				return
			}

			log.Info(
				"upcoming matches sync completed",
				zap.String("trigger", trigger),
				zap.Int("saved", saved),
			)
		}

		runSync("startup")
		ticker := time.NewTicker(interval)
		go func() {
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					runSync("ticker")
				}
			}
		}()
	}

	grpcApp := grpcapp.New(log, cfg.GRPC.Host, cfg.GRPC.Port, func(s *grpc.Server) {
		grpcapi.Register(s, log, matchService)
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- grpcApp.Run()
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
		grpcApp.Stop()
	case err := <-errCh:
		if err != nil {
			log.Error("gRPC server stopped", zap.Error(err))
		}
	}
}

func setupLogger(level string) *zap.Logger {
	zapLevel := parseLogLevel(level)
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)

	log, err := cfg.Build()
	if err != nil {
		panic(err)
	}

	return log
}

func parseLogLevel(level string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zapcore.DebugLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
