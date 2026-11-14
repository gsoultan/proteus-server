package pgstore_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gsoultan/storm/runtime/pgxdrv"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gsoultan/proteus-server/internal/jobs/storetest"
	"github.com/gsoultan/proteus-server/internal/storage/pgstore"
	tplstoretest "github.com/gsoultan/proteus-server/internal/templates/storetest"
)

// TestStore runs the job and template store conformance suites against a real PostgreSQL in a
// schema of its own. Set PROTEUS_TEST_DATABASE_URL to run it.
func TestStore(t *testing.T) {
	dsn := os.Getenv("PROTEUS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PROTEUS_TEST_DATABASE_URL unset")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("proteus_test_%d", time.Now().UnixNano())
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxdrv.NewPoolConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pgstore.Migrate(ctx, pool, schema, t.Logf); err != nil {
		t.Fatal(err)
	}
	// A second migration finds nothing to do.
	if err := pgstore.Migrate(ctx, pool, schema, t.Logf); err != nil {
		t.Fatal(err)
	}
	store := pgstore.New(pool)
	t.Run("Jobs", func(t *testing.T) { storetest.Run(t, store) })
	t.Run("Templates", func(t *testing.T) { tplstoretest.Run(t, store.Templates()) })
}
