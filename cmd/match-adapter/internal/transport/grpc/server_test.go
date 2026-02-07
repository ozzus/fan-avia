package grpc

import (
	"context"
	"errors"
	"testing"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapGetMatchError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "not_found", err: derr.ErrMatchNotFound, code: codes.NotFound},
		{name: "unavailable", err: derr.ErrSourceUnavailable, code: codes.Unavailable},
		{name: "deadline", err: context.DeadlineExceeded, code: codes.DeadlineExceeded},
		{name: "canceled", err: context.Canceled, code: codes.Canceled},
		{name: "internal", err: errors.New("boom"), code: codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGetMatchError(tt.err)
			if status.Code(got) != tt.code {
				t.Fatalf("expected code %s, got %s", tt.code, status.Code(got))
			}
		})
	}
}
