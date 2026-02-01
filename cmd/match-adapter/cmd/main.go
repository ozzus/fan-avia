package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	grpcapi "github.com/ozzus/fan-avia/cmd/match-adapter/internal/api/grpc"
	"github.com/ozzus/fan-avia/cmd/match-adapter/grpcapp"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.MustLoad()
	log := setupLogger(cfg.Log.Level)
	defer func() {
		_ = log.Sync()
	}()

	log.Info("match-adapter starting", zap.String("grpc_addr", fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port)))

	app := grpcapp.New(log, cfg.GRPC.Host, cfg.GRPC.Port, func(s *grpc.Server) {
		grpcapi.Register(s, log)
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
