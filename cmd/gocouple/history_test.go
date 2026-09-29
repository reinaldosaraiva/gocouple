package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestHistoryCommand(t *testing.T) {
	repo := t.TempDir()
	gitIn(t, repo, "init", "-b", "main")
	write := func(name, body string) {
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/h\n\ngo 1.26\n")
	write("a/a.go", "package a\n")
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-m", "one")
	write("b/b.go", "package b\n\nimport _ \"example.com/h/a\"\n")
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-m", "two")

	out := filepath.Join(t.TempDir(), "history.json")
	csv := filepath.Join(t.TempDir(), "history.csv")
	args := []string{"history", "--repo", repo, "--out", out, "--csv", csv, "--workers", "2"}
	if _, err := runCLI(t, args...); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var h model.History
	if err := json.Unmarshal(body, &h); err != nil {
		t.Fatal(err)
	}
	if len(h.Snapshots) != 2 || h.Snapshots[0].Commit.Subject != "one" || h.Snapshots[1].Snapshot.Summary.Packages != 2 {
		t.Errorf("history = %+v", h)
	}
	rows, _ := os.ReadFile(csv)
	if !strings.HasPrefix(string(rows), "sha,date,package,") || strings.Count(string(rows), "\n") != 4 {
		t.Errorf("csv:\n%s", rows)
	}
	if _, err := os.Stat(filepath.Join(repo, ".gocouple", "cache")); err != nil {
		t.Errorf("cache directory missing: %v", err)
	}
	if _, err := runCLI(t, "report", "--history", out, "--out", filepath.Join(t.TempDir(), "r.html")); err != nil {
		t.Errorf("report from the generated history: %v", err)
	}
}

func TestHistoryFailsOutsideRepository(t *testing.T) {
	if _, err := runCLI(t, "history", "--repo", t.TempDir(), "--out", filepath.Join(t.TempDir(), "h.json")); err == nil {
		t.Error("expected error outside a git repository")
	}
}
