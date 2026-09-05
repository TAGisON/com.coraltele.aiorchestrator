package cihygiene

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from", wd)
		}
		dir = parent
	}
}

func TestGitignoreListsSecretsLocal(t *testing.T) {
	root := repoRoot(t)
	f, err := os.Open(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == ".agent/secrets.local.json" {
			return
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	t.Fatal(".gitignore missing exact line .agent/secrets.local.json")
}

func TestForbiddenSecretPathsNotTracked(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("git", "ls-files",
		".agent/secrets.local.json",
		".agent/secrets",
		".env",
		"credentials.json",
	)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-files: %v\n%s", err, out)
	}
	tracked := strings.TrimSpace(string(out))
	if tracked != "" {
		t.Fatalf("forbidden secret-related paths are tracked:\n%s", tracked)
	}
}

func TestSecretsLocalIsIgnoredWhenPresent(t *testing.T) {
	root := repoRoot(t)
	// check-ignore exits 0 if ignored; 1 if not ignored / missing rules.
	cmd := exec.Command("git", "check-ignore", "-q", ".agent/secrets.local.json")
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("git check-ignore .agent/secrets.local.json failed (must be ignored): %v", err)
	}
}

func TestSecretsExampleIsPlaceholderOnly(t *testing.T) {
	root := repoRoot(t)
	p := filepath.Join(root, ".agent", "secrets.example.json")
	body, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("no secrets.example.json")
		}
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("YOUR_SARVAM_API_KEY")) {
		t.Fatal("secrets.example.json must keep YOUR_SARVAM_API_KEY placeholder")
	}
	if bytes.Contains(bytes.ToLower(body), []byte("sk-live")) {
		t.Fatal("secrets.example.json must not contain sk-live material")
	}
}
