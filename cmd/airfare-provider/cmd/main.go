package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/grpcapp"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/application/service"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/config"
	cacheredis "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/infrastructures/db/redis"
	airfaretracing "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/infrastructures/db/tracing"
	matchclient "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/infrastructures/match"
	tpclient "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/infrastructures/travelpayouts/http/client"
	grpcapi "github.com/ozzus/fan-avia/cmd/airfare-provider/internal/transport/grpc"
	matchv1 "github.com/ozzus/fan-avia/protos/gen/go/match/v1"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	_ = godotenv.Load(".env")

	cfg := config.MustLoad()
	log := setupLogger(cfg.Log.Level)
	defer func() {
		_ = log.Sync()
	}()

	tp, err := airfaretracing.InitTracer("airfare-provider", cfg.Jaeger)
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

	log.Info("airfare-provider starting", zap.String("grpc_addr", fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port)))

	matchConn, err := grpc.NewClient(cfg.MatchAdapter.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("failed to connect match-adapter grpc", zap.Error(err), zap.String("addr", cfg.MatchAdapter.Address()))
	}
	defer func() {
		if err := matchConn.Close(); err != nil {
			log.Warn("failed to close match-adapter grpc client", zap.Error(err))
		}
	}()

	matchReader := matchclient.NewClient(matchv1.NewMatchAdapterServiceClient(matchConn), cfg.MatchAdapter.Timeout)
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

	airfareCache := cacheredis.NewAirfareCacheRepository(redisClient)
	fareSource := tpclient.NewClient(
		cfg.Travelpayouts.BaseURL,
		cfg.Travelpayouts.Token,
		cfg.Travelpayouts.Currency,
		cfg.Travelpayouts.Limit,
		cfg.Travelpayouts.Timeout,
	)
	airfareService := service.NewAirfareService(log, matchReader, fareSource, airfareCache, cfg.AirfareCacheTTL)

	app := grpcapp.New(log, cfg.GRPC.Host, cfg.GRPC.Port, func(s *grpc.Server) {
		grpcapi.Register(s, log, airfareService)
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Run()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
		app.Stop()
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
