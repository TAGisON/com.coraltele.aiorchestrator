package bootconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPort8011(t *testing.T) {
	c := Default()
	if c.HTTPAddr != ":8011" {
		t.Fatalf("http addr %q", c.HTTPAddr)
	}
}

func TestMergeFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.properties")
	body := "http.addr=:9000\ndatabase.require=true\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":9000" {
		t.Fatalf("addr %q", cfg.HTTPAddr)
	}
	if !cfg.RequireDatabase {
		t.Fatal("require db")
	}
	t.Setenv("HTTP_ADDR", ":8011")
	cfg2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.HTTPAddr != ":8011" {
		t.Fatalf("env override %q", cfg2.HTTPAddr)
	}
}
