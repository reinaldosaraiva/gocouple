package history_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reinaldosaraiva/gocouple/internal/gitrun"

	"github.com/reinaldosaraiva/gocouple/internal/analysis"
	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/history"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

func git(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

const (
	godTree   = "hub"
	painTree  = "logger"
	fanoutPkg = 8
)

// tree returns the module files for each step of the scenario: a clean
// pair, a god package, a concrete hotspot on top, and a refactoring.
func tree(step int) map[string]string {
	files := map[string]string{
		"go.mod": "module example.com/h\n\ngo 1.26\n",
		"a/a.go": "package a\n\nimport _ \"example.com/h/b\"\n",
		"b/b.go": "package b\n\ntype T struct{}\n",
	}
	if step >= 1 {
		hub := "package hub\n\nimport (\n"
		for i := 1; i <= fanoutPkg; i++ {
			name := fmt.Sprintf("p%d", i)
			files[name+"/"+name+".go"] = "package " + name + "\n\ntype T struct{}\n"
			hub += fmt.Sprintf("\t_ \"example.com/h/%s\"\n", name)
		}
		files[godTree+"/hub.go"] = hub + ")\n"
	}
	if step >= 2 {
		files[painTree+"/logger.go"] = "package logger\n\ntype L struct{}\n"
		for i := 1; i <= 6; i++ {
			name := fmt.Sprintf("p%d", i)
			files[name+"/"+name+".go"] = "package " + name + "\n\nimport _ \"example.com/h/logger\"\n\ntype T struct{}\n"
		}
	}
	if step == 3 {
		delete(files, godTree+"/hub.go")
		delete(files, painTree+"/logger.go")
		for i := 1; i <= 6; i++ {
			name := fmt.Sprintf("p%d", i)
			files[name+"/"+name+".go"] = "package " + name + "\n\ntype T struct{}\n"
		}
	}
	return files
}

func commitStep(t *testing.T, repo string, step int) {
	t.Helper()
	entries, err := os.ReadDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != ".git" {
			if err := os.RemoveAll(filepath.Join(repo, e.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	for name, body := range tree(step) {
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	date := fmt.Sprintf("2026-02-0%dT12:00:00Z", step+1)
	env := []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}
	git(t, repo, nil, "add", "-A")
	git(t, repo, env, "commit", "-m", fmt.Sprintf("step %d", step))
}

func worktrees(t *testing.T, repo string) int {
	t.Helper()
	n := 0
	for line := range strings.SplitSeq(git(t, repo, nil, "worktree", "list", "--porcelain"), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			n++
		}
	}
	return n
}

func ids(snap *model.Snapshot) map[string]bool {
	out := map[string]bool{}
	for _, d := range snap.Diagnostics {
		out[d.ID] = true
	}
	return out
}

func TestHistoryOnRealRepository(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, nil, "init", "-b", "main")
	for step := range 4 {
		commitStep(t, repo, step)
	}
	dirty := filepath.Join(repo, "a", "a.go")
	if err := os.WriteFile(dirty, []byte("package a\n\n// uncommitted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statusBefore := git(t, repo, nil, "status", "--porcelain")

	cfg := config.Default()
	analyze := func(ctx context.Context, dir string) (*model.Snapshot, error) {
		return analysis.Run(ctx, analysis.Options{Dir: dir, Config: cfg, ToolVersion: "test"})
	}
	var progress bytes.Buffer
	opts := history.Options{
		Repo:      repo,
		Selection: history.Selection{FirstParent: true, Max: 50},
		Workers:   2,
		CacheDir:  filepath.Join(repo, ".gocouple", "cache"),
		CacheKey:  history.ConfigHash("test", cfg, false, nil),
		Progress:  &progress,
	}

	h, stats, err := history.Run(t.Context(), gitrun.Exec{}, analyze, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Snapshots) != 4 || stats.Analyzed != 4 || stats.Cached != 0 || stats.Failed != 0 {
		t.Fatalf("snapshots=%d stats=%+v", len(h.Snapshots), stats)
	}
	if h.Module != "example.com/h" {
		t.Errorf("module = %q", h.Module)
	}

	wantPackages := []int{2, 11, 12, 10}
	wantGod := []bool{false, true, true, false}
	wantPain := []bool{false, false, true, false}
	for i, e := range h.Snapshots {
		if e.Snapshot == nil {
			t.Fatalf("entry %d failed: %s", i, e.Error)
		}
		if e.Snapshot.Summary.Packages != wantPackages[i] {
			t.Errorf("step %d: packages = %d, want %d", i, e.Snapshot.Summary.Packages, wantPackages[i])
		}
		got := ids(e.Snapshot)
		if got["god-package"] != wantGod[i] || got["pain-zone"] != wantPain[i] {
			t.Errorf("step %d: diagnostics = %v", i, got)
		}
		if e.Commit.Subject != fmt.Sprintf("step %d", i) {
			t.Errorf("entry %d is %q: history must be oldest first", i, e.Commit.Subject)
		}
	}
	if h.Snapshots[3].Snapshot.Summary.Pain >= h.Snapshots[2].Snapshot.Summary.Pain {
		t.Errorf("pain by commit: %d then %d", h.Snapshots[2].Snapshot.Summary.Pain, h.Snapshots[3].Snapshot.Summary.Pain)
	}

	if n := worktrees(t, repo); n != 1 {
		t.Errorf("worktrees after run = %d, want only the main one", n)
	}
	if got := strings.ReplaceAll(git(t, repo, nil, "status", "--porcelain"), "?? .gocouple/\n", ""); got != statusBefore {
		t.Errorf("working tree changed:\nbefore %q\nafter  %q", statusBefore, got)
	}
	if body, _ := os.ReadFile(dirty); !strings.Contains(string(body), "uncommitted") {
		t.Error("uncommitted change was lost")
	}

	again, stats2, err := history.Run(t.Context(), gitrun.Exec{}, analyze, opts)
	if err != nil {
		t.Fatal(err)
	}
	if stats2.Cached != 4 || stats2.Analyzed != 0 || len(again.Snapshots) != 4 {
		t.Errorf("second run stats = %+v", stats2)
	}
	if !strings.Contains(progress.String(), "(cached)") || !strings.Contains(progress.String(), "[4/4]") {
		t.Errorf("progress output:\n%s", progress.String())
	}
}

func TestHistoryCancelledLeavesNoWorktree(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, nil, "init", "-b", "main")
	for step := range 3 {
		commitStep(t, repo, step)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	analyze := func(ctx context.Context, dir string) (*model.Snapshot, error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	}
	_, _, err := history.Run(ctx, gitrun.Exec{}, analyze, history.Options{
		Repo: repo, Selection: history.Selection{FirstParent: true}, Workers: 2, NoCache: true,
	})
	if err == nil {
		t.Fatal("expected an interruption error")
	}
	if n := worktrees(t, repo); n != 1 {
		t.Errorf("worktrees after cancellation = %d, want 1", n)
	}
}
