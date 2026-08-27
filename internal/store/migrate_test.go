package store_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestMigrationSQL_HasPhaseBTables(t *testing.T) {
	// Ensure embedded migration source lists all Phase B tables (compile-time embed).
	// Full apply is covered by TestApplyMigrations_Integration when DATABASE_URL is set.
	body, err := os.ReadFile("migrations/001_phase_b.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, table := range []string{
		"profile", "profile_version", "session", "audit_event", "playback_job", "postcall_job",
	} {
		if !strings.Contains(s, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("migration missing table %s", table)
		}
	}
}

func TestApplyMigrations_Integration(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skip PG migration integration")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyMigrations(ctx, st.Pool()); err != nil {
		t.Fatal(err)
	}
}
