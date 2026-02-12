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

	apihandlers "github.com/ozzus/fan-avia/cmd/api-gateway/internal/api/http/handlers"
	airfareclient "github.com/ozzus/fan-avia/cmd/api-gateway/internal/clients/airfare"
	matchclient "github.com/ozzus/fan-avia/cmd/api-gateway/internal/clients/match"
	"github.com/ozzus/fan-avia/cmd/api-gateway/internal/config"
	airfarev1 "github.com/ozzus/fan-avia/protos/gen/go/airfare/v1"
	matchv1 "github.com/ozzus/fan-avia/protos/gen/go/match/v1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := config.MustLoad()
	log := setupLogger(cfg.Log.Level)
	defer func() {
		_ = log.Sync()
	}()

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	log.Info("api-gateway starting", zap.String("http_addr", addr))

	airfareConn, err := grpc.NewClient(cfg.Clients.Airfare.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("failed to connect airfare-provider grpc", zap.Error(err), zap.String("addr", cfg.Clients.Airfare.Address))
	}
	defer func() {
		if closeErr := airfareConn.Close(); closeErr != nil {
			log.Warn("failed to close airfare-provider grpc client", zap.Error(closeErr))
		}
	}()

	airfareClient := airfareclient.NewClient(
		airfarev1.NewAirfareProviderServiceClient(airfareConn),
		cfg.Clients.Airfare.Timeout,
	)
	matchConn, err := grpc.NewClient(cfg.Clients.Match.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("failed to connect match-adapter grpc", zap.Error(err), zap.String("addr", cfg.Clients.Match.Address))
	}
	defer func() {
		if closeErr := matchConn.Close(); closeErr != nil {
			log.Warn("failed to close match-adapter grpc client", zap.Error(closeErr))
		}
	}()
	matchClient := matchclient.NewClient(
		matchv1.NewMatchAdapterServiceClient(matchConn),
		cfg.Clients.Match.Timeout,
	)

	airfareHandler := apihandlers.NewAirfareHandler(log, airfareClient, cfg.Clients.Airfare.Timeout, cfg.Defaults.OriginIATA)
	matchHandler := apihandlers.NewMatchHandler(log, matchClient, cfg.Clients.Match.Timeout)
	catalogTimeout := 20 * time.Second
	if cfg.HTTP.WriteTimeout > 0 {
		adjusted := cfg.HTTP.WriteTimeout - 500*time.Millisecond
		if adjusted < time.Second {
			adjusted = time.Second
		}
		if adjusted < catalogTimeout {
			catalogTimeout = adjusted
		}
	}
	catalogHandler := apihandlers.NewCatalogHandler(log, matchClient, airfareClient, catalogTimeout, cfg.Defaults.OriginIATA)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/v1/stub", stubHandler(log))
	mux.HandleFunc("/v1/matches", matchHandler.GetMatches)
	mux.HandleFunc("/v1/matches/upcoming", matchHandler.GetUpcomingMatches)
	mux.HandleFunc("/v1/matches/upcoming-with-airfare", catalogHandler.GetUpcomingWithAirfare)
	mux.HandleFunc("/v1/matches/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/airfare") {
			airfareHandler.GetAirfareByMatch(w, r)
			return
		}
		matchHandler.GetMatch(w, r)
	})

	server := &http.Server{
		Addr:         addr,
		Handler:      loggingMiddleware(log, mux),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("http shutdown error", zap.Error(err))
		}
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Error("http server stopped", zap.Error(err))
		}
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func stubHandler(log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("stub request", zap.String("method", r.Method), zap.String("path", r.URL.Path))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

func loggingMiddleware(log *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Info("http request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Duration("duration", time.Since(start)),
		)
	})
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
