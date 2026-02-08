package grpcapp

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type GrpcApp struct {
	log        *zap.Logger
	gRPCServer *grpc.Server
	addr       string
}

func New(log *zap.Logger, host string, port int, register func(*grpc.Server)) *GrpcApp {
	addr := fmt.Sprintf("%s:%d", host, port)

	gRPCServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			tracingInterceptor(),
			recoveryInterceptor(log),
			loggingInterceptor(log),
		),
	)

	register(gRPCServer)

	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(gRPCServer, healthServer)

	reflection.Register(gRPCServer)

	return &GrpcApp{
		log:        log,
		gRPCServer: gRPCServer,
		addr:       addr,
	}
}

func tracingInterceptor() grpc.UnaryServerInterceptor {
	tracer := otel.Tracer("match-adapter/grpc")

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			ctx = otel.GetTextMapPropagator().Extract(ctx, metadataCarrier(md))
		}

		ctx, span := tracer.Start(ctx, info.FullMethod, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		return handler(ctx, req)
	}
}

type metadataCarrier metadata.MD

func (c metadataCarrier) Get(key string) string {
	values := metadata.MD(c).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (c metadataCarrier) Set(key, value string) {
	metadata.MD(c).Set(key, value)
}

func (c metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

func (a *GrpcApp) Run() error {
	const op = "grpcapp.Run"

	l, err := net.Listen("tcp", a.addr)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	a.log.Info("gRPC server started", zap.String("addr", l.Addr().String()))

	if err := a.gRPCServer.Serve(l); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *GrpcApp) Stop() {
	a.log.Info("stopping gRPC server", zap.String("addr", a.addr))
	a.gRPCServer.GracefulStop()
}

func loggingInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		code := status.Code(err)
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.String("code", code.String()),
			zap.Duration("duration", time.Since(start)),
		}

		if err != nil {
			log.Error("gRPC request failed", append(fields, zap.Error(err))...)
			return resp, err
		}

		log.Info("gRPC request", fields...)
		return resp, nil
	}
}

func recoveryInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered", zap.Any("panic", r), zap.String("method", info.FullMethod))
				err = status.Error(codes.Internal, "internal error")
			}
		}()

		return handler(ctx, req)
	}
}
