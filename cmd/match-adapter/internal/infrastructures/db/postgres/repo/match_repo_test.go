package postgres

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestBuildPoolConfig_DisablesPreparedStatements(t *testing.T) {
	t.Parallel()

	cfg, err := buildPoolConfig("postgresql://user:pass@localhost:5432/postgres?sslmode=disable")
	if err != nil {
		t.Fatalf("buildPoolConfig returned error: %v", err)
	}

	if cfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeSimpleProtocol {
		t.Fatalf("unexpected query exec mode: got %v", cfg.ConnConfig.DefaultQueryExecMode)
	}
	if cfg.ConnConfig.StatementCacheCapacity != 0 {
		t.Fatalf("unexpected statement cache capacity: got %d", cfg.ConnConfig.StatementCacheCapacity)
	}
	if cfg.ConnConfig.DescriptionCacheCapacity != 0 {
		t.Fatalf("unexpected description cache capacity: got %d", cfg.ConnConfig.DescriptionCacheCapacity)
	}
}
