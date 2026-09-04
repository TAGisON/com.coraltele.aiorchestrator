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

func TestMigrationSQL_HasCCTenantEngines(t *testing.T) {
	body, err := os.ReadFile("migrations/004_cc_tenant_engines.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "CREATE TABLE IF NOT EXISTS tenant_engines") {
		t.Fatal("migration missing tenant_engines")
	}
	if !strings.Contains(s, "gateway_binding") {
		t.Fatal("migration missing session.gateway_binding")
	}
}

func TestMigrationSQL_HasPhaseEAnalytics(t *testing.T) {
	body, err := os.ReadFile("migrations/003_phase_e_analytics.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "CREATE TABLE IF NOT EXISTS analytics_event") {
		t.Fatal("migration missing analytics_event")
	}
}

func TestMigrationSQL_HasPhaseDKBTables(t *testing.T) {
	body, err := os.ReadFile("migrations/002_phase_d_kb.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, table := range []string{"kb_document", "kb_chunk"} {
		if !strings.Contains(s, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("migration missing table %s", table)
		}
	}
}

func TestMigrationSQL_HasCCLanguagePolicy(t *testing.T) {
	body, err := os.ReadFile("migrations/005_cc_language_policy.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "detected_language") || !strings.Contains(s, "active_language") {
		t.Fatal("migration missing session language columns")
	}
}

func TestMigrationSQL_HasCCTranscriptDisposition(t *testing.T) {
	body, err := os.ReadFile("migrations/006_cc_transcript_disposition.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "CREATE TABLE IF NOT EXISTS transcript_turn") {
		t.Fatal("migration missing transcript_turn")
	}
	if !strings.Contains(s, "CREATE TABLE IF NOT EXISTS session_disposition") {
		t.Fatal("migration missing session_disposition")
	}
}

func TestMigrationSQL_HasGatewayCredentials(t *testing.T) {
	body, err := os.ReadFile("migrations/007_gateway_credentials.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "CREATE TABLE IF NOT EXISTS gateway_credentials") {
		t.Fatal("migration missing gateway_credentials")
	}
	if !strings.Contains(s, "CREATE TABLE IF NOT EXISTS system_settings") {
		t.Fatal("migration missing system_settings")
	}
}

func TestMigrationSQL_HasCallerPreference(t *testing.T) {
	body, err := os.ReadFile("migrations/009_caller_preference.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "CREATE TABLE IF NOT EXISTS caller_preference") {
		t.Fatal("migration missing caller_preference")
	}
	if !strings.Contains(s, "preferred_language") {
		t.Fatal("migration missing preferred_language column")
	}
}

func TestMigrationSQL_HasFlowRegistry(t *testing.T) {
	body, err := os.ReadFile("migrations/010_flow_registry.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, table := range []string{"flow", "flow_draft", "flow_version"} {
		if !strings.Contains(s, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("migration missing table %s", table)
		}
	}
	if strings.Contains(s, "CREATE TABLE IF NOT EXISTS desk") {
		t.Fatal("flow registry must not create desk tables")
	}
	if strings.Contains(s, "ALTER TABLE session") {
		t.Fatal("M-A must not ALTER session (pins are M-C)")
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
