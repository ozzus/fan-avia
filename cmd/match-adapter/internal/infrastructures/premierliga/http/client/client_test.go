package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
)

func TestGetFullDataMatch_404OnFullDataEndpointMapsToNotFound(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	c := NewClient(srv.URL+"/api/getFullDataMatch", srv.Client(), 1, 100*time.Millisecond)
	_, err := c.GetFullDataMatch(context.Background(), 16114)
	if !errors.Is(err, derr.ErrMatchNotFound) {
		t.Fatalf("expected ErrMatchNotFound, got %v", err)
	}
}

func TestGetFullDataMatch_404OnWrongEndpointMapsToUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	c := NewClient(srv.URL+"/api/getFullDataMatch-broken", srv.Client(), 1, 100*time.Millisecond)
	_, err := c.GetFullDataMatch(context.Background(), 16114)
	if !errors.Is(err, derr.ErrSourceUnavailable) {
		t.Fatalf("expected ErrSourceUnavailable, got %v", err)
	}
}
