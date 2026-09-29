package history

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

type fakeGit struct {
	mu        sync.Mutex
	log       []string
	commits   []model.Commit
	active    int
	overlap   bool
	adds      int
	removes   int
	prunes    int
	failAddOn string
}

func (f *fakeGit) Run(_ context.Context, dir string, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.log = append(f.log, strings.Join(args, " "))
	f.mu.Unlock()
	switch args[0] {
	case "rev-parse":
		return []byte(dir + "\n"), nil
	case "log":
		var b strings.Builder
		for i := len(f.commits) - 1; i >= 0; i-- {
			c := f.commits[i]
			fmt.Fprintf(&b, "%s\x1f%s\x1f%s\n", c.SHA, c.Date, c.Subject)
		}
		return []byte(b.String()), nil
	case "worktree":
		return nil, f.worktree(args)
	}
	return nil, fmt.Errorf("unexpected git %v", args)
}

func (f *fakeGit) worktree(args []string) error {
	f.mu.Lock()
	f.active++
	if f.active > 1 {
		f.overlap = true
	}
	f.mu.Unlock()
	for range 20 {
		runtime.Gosched()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active--
	switch args[1] {
	case "add":
		if f.failAddOn != "" && args[len(args)-1] == f.failAddOn {
			return errors.New("add refused")
		}
		f.adds++
	case "remove":
		f.removes++
	case "prune":
		f.prunes++
	}
	return nil
}

func makeCommits(n int) []model.Commit {
	out := make([]model.Commit, n)
	for i := range out {
		out[i] = model.Commit{
			SHA:     fmt.Sprintf("%040d", i+1),
			Date:    time.Date(2026, 1, i+1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
			Subject: fmt.Sprintf("commit %d", i+1),
		}
	}
	return out
}

func okAnalyzer(_ context.Context, _ string) (*model.Snapshot, error) {
	return &model.Snapshot{SchemaVersion: model.SchemaVersion, Module: "example.com/m"}, nil
}

func opts(t *testing.T) Options {
	t.Helper()
	repo := t.TempDir()
	return Options{Repo: repo, Dir: repo, Workers: 3, NoCache: true, Selection: Selection{FirstParent: true}}
}

func TestListCommitsSampling(t *testing.T) {
	git := &fakeGit{commits: makeCommits(10)}
	tests := []struct {
		name string
		sel  Selection
		want []int
	}{
		{"all oldest first", Selection{}, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
		{"max keeps newest", Selection{Max: 3}, []int{8, 9, 10}},
		{"every 3 from newest", Selection{Every: 3}, []int{1, 4, 7, 10}},
		{"every and max", Selection{Every: 3, Max: 2}, []int{7, 10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ListCommits(t.Context(), git, "/repo", tt.sel)
			if err != nil {
				t.Fatal(err)
			}
			var idx []int
			for _, c := range got {
				n, err := strconv.Atoi(strings.TrimLeft(c.SHA, "0"))
				if err != nil {
					t.Fatal(err)
				}
				idx = append(idx, n)
			}
			if diff := cmp.Diff(tt.want, idx); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestListCommitsArguments(t *testing.T) {
	git := &fakeGit{commits: makeCommits(1)}
	if _, err := ListCommits(t.Context(), git, "/repo", Selection{Branch: "main", Since: "2026-01-01", Until: "2026-02-01", FirstParent: true}); err != nil {
		t.Fatal(err)
	}
	want := "log --format=%H%x1f%cI%x1f%s --first-parent --since=2026-01-01 --until=2026-02-01 main --"
	if git.log[0] != want {
		t.Errorf("git args = %q, want %q", git.log[0], want)
	}
}

func TestParseLogNormalizesDatesToUTC(t *testing.T) {
	got, err := parseLog("abc\x1f2026-03-10T12:00:00-03:00\x1fsubject with \x1f no split\n")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Date != "2026-03-10T15:00:00Z" || got[0].Subject != "subject with \x1f no split" {
		t.Errorf("commit = %+v", got[0])
	}
	if _, err := parseLog("broken line\n"); err == nil {
		t.Error("expected error for a malformed line")
	}
}

func TestRunOrderedAndCleansUp(t *testing.T) {
	git := &fakeGit{commits: makeCommits(6)}
	h, stats, err := Run(t.Context(), git, okAnalyzer, opts(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Snapshots) != 6 || h.Module != "example.com/m" || h.SchemaVersion != "1" {
		t.Fatalf("history = %+v", h)
	}
	for i, e := range h.Snapshots {
		if e.Snapshot == nil || e.Snapshot.Commit.SHA != makeCommits(6)[i].SHA {
			t.Errorf("entry %d out of order: %+v", i, e.Commit)
		}
	}
	if git.adds != 6 || git.removes != 6 || git.prunes != 1 || git.overlap {
		t.Errorf("adds=%d removes=%d prunes=%d overlap=%v", git.adds, git.removes, git.prunes, git.overlap)
	}
	if stats.Analyzed != 6 || stats.Cached != 0 || stats.Failed != 0 {
		t.Errorf("stats = %+v", stats)
	}
}

func TestRunErrorEntryDoesNotAbort(t *testing.T) {
	git := &fakeGit{commits: makeCommits(4)}
	bad := makeCommits(4)[1].SHA
	analyze := func(_ context.Context, dir string) (*model.Snapshot, error) {
		return okAnalyzer(context.Background(), dir)
	}
	git.failAddOn = bad
	var progress bytes.Buffer
	o := opts(t)
	o.Progress = &progress
	h, stats, err := Run(t.Context(), git, analyze, o)
	if err != nil {
		t.Fatal(err)
	}
	if h.Snapshots[1].Error == "" || h.Snapshots[1].Snapshot != nil || h.Snapshots[1].Commit.SHA != bad {
		t.Errorf("failed entry = %+v", h.Snapshots[1])
	}
	if stats.Failed != 1 || stats.Analyzed != 3 {
		t.Errorf("stats = %+v", stats)
	}
	if git.removes != git.adds {
		t.Errorf("adds=%d removes=%d", git.adds, git.removes)
	}
	out := progress.String()
	if !strings.Contains(out, "/4]") || !strings.Contains(out, "(error: ") {
		t.Errorf("progress output:\n%s", out)
	}
}

func TestAnalyzerErrorBecomesEntry(t *testing.T) {
	git := &fakeGit{commits: makeCommits(2)}
	analyze := func(_ context.Context, _ string) (*model.Snapshot, error) { return nil, errors.New("does not compile") }
	h, stats, err := Run(t.Context(), git, analyze, opts(t))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Failed != 2 || !strings.Contains(h.Snapshots[0].Error, "does not compile") {
		t.Errorf("stats=%+v entry=%+v", stats, h.Snapshots[0])
	}
	if git.removes != 2 {
		t.Errorf("removes = %d", git.removes)
	}
}

func TestCacheHitsAndInvalidation(t *testing.T) {
	git := &fakeGit{commits: makeCommits(3)}
	calls := 0
	var mu sync.Mutex
	analyze := func(ctx context.Context, dir string) (*model.Snapshot, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return okAnalyzer(ctx, dir)
	}
	o := opts(t)
	o.NoCache = false
	o.CacheDir = filepath.Join(t.TempDir(), "cache")
	o.CacheKey = ConfigHash("v1", config.Default(), false, nil)

	first, s1, err := Run(t.Context(), git, analyze, o)
	if err != nil || s1.Analyzed != 3 || s1.Cached != 0 {
		t.Fatalf("first run: stats=%+v err=%v", s1, err)
	}
	second, s2, err := Run(t.Context(), git, analyze, o)
	if err != nil || s2.Analyzed != 0 || s2.Cached != 3 || calls != 3 {
		t.Fatalf("second run: stats=%+v calls=%d err=%v", s2, calls, err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if !bytes.Equal(a, b) {
		t.Error("cached run differs from the fresh run")
	}

	changed := config.Default()
	changed.GodCeThreshold = 3
	o.CacheKey = ConfigHash("v1", changed, false, nil)
	if _, s3, _ := Run(t.Context(), git, analyze, o); s3.Cached != 0 {
		t.Errorf("config change must miss the cache: %+v", s3)
	}

	o.CacheKey = ConfigHash("v1", config.Default(), false, nil)
	entries, _ := filepath.Glob(filepath.Join(o.CacheDir, "*-"+cacheKey(o.CacheKey, ".")+".json"))
	if len(entries) != 3 {
		t.Fatalf("cache entries = %v", entries)
	}
	if err := os.WriteFile(entries[0], []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, s4, err := Run(t.Context(), git, analyze, o); err != nil || s4.Cached != 2 || s4.Analyzed != 1 {
		t.Errorf("corrupt entry must be recomputed: stats=%+v err=%v", s4, err)
	}

	o.NoCache = true
	if _, s5, _ := Run(t.Context(), git, analyze, o); s5.Cached != 0 {
		t.Errorf("--no-cache must skip the cache: %+v", s5)
	}
}

func TestConfigHashSensitivity(t *testing.T) {
	base := ConfigHash("v1", config.Default(), false, []string{"./..."})
	if len(base) != 12 {
		t.Fatalf("hash = %q", base)
	}
	other := config.Default()
	other.Exclude = []string{"**/x/**"}
	for name, h := range map[string]string{
		"version":  ConfigHash("v2", config.Default(), false, []string{"./..."}),
		"config":   ConfigHash("v1", other, false, []string{"./..."}),
		"tests":    ConfigHash("v1", config.Default(), true, []string{"./..."}),
		"patterns": ConfigHash("v1", config.Default(), false, []string{"./x"}),
	} {
		if h == base {
			t.Errorf("hash ignores %s", name)
		}
	}
	if ConfigHash("v1", config.Default(), false, []string{"./..."}) != base {
		t.Error("hash is not stable")
	}
}

func TestCancellationRemovesEveryWorktree(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		git := &fakeGit{commits: makeCommits(8)}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		started := make(chan struct{}, 8)
		analyze := func(ctx context.Context, _ string) (*model.Snapshot, error) {
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		}

		type result struct {
			h   *model.History
			err error
		}
		done := make(chan result, 1)
		go func() {
			h, _, err := Run(ctx, git, analyze, opts(t))
			done <- result{h, err}
		}()
		for range 3 {
			<-started
		}
		synctest.Wait()
		cancel()
		res := <-done

		if !errors.Is(res.err, context.Canceled) || res.h != nil {
			t.Fatalf("err=%v history=%v", res.err, res.h)
		}
		if git.adds == 0 || git.adds != git.removes || git.prunes != 1 {
			t.Errorf("adds=%d removes=%d prunes=%d", git.adds, git.removes, git.prunes)
		}
	})
}

func TestEntryJSONRoundTrip(t *testing.T) {
	c := model.Commit{SHA: "abc", Date: "2026-01-01T00:00:00Z", Subject: "s"}
	h := model.History{SchemaVersion: "1", Module: "m", Snapshots: []model.Entry{
		{Commit: c, Snapshot: &model.Snapshot{SchemaVersion: "1", Module: "m", Commit: &c}},
		{Commit: c, Error: "boom"},
	}}
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `{"commit":{"sha":"abc","date":"2026-01-01T00:00:00Z","subject":"s"},"error":"boom"}`) {
		t.Errorf("failed entry shape: %s", data)
	}
	var back model.History
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Snapshots[0].Snapshot == nil || back.Snapshots[0].Snapshot.Module != "m" || back.Snapshots[1].Error != "boom" || back.Snapshots[1].Snapshot != nil {
		t.Errorf("round trip = %+v", back)
	}
}

func TestModuleRel(t *testing.T) {
	top := t.TempDir()
	sub := filepath.Join(top, "svc", "api")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if rel, err := moduleRel(top, sub, top); err != nil || rel != filepath.Join("svc", "api") {
		t.Errorf("rel=%q err=%v", rel, err)
	}
	if rel, err := moduleRel(top, "", top); err != nil || rel != "." {
		t.Errorf("default rel=%q err=%v", rel, err)
	}
	if _, err := moduleRel(top, filepath.Dir(top), top); err == nil {
		t.Error("directory outside the repository must fail")
	}
}

func TestCacheKeySeparatesModules(t *testing.T) {
	if cacheKey("k", "svc/a") == cacheKey("k", "svc/b") || cacheKey("k", ".") == cacheKey("k2", ".") {
		t.Error("cache key must depend on module directory and key")
	}
	first, second := cacheKey("k", "svc/a"), cacheKey("k", "svc/a")
	if first != second {
		t.Error("cache key must be stable")
	}
}

func TestModuleRelAcceptsDotDotPrefixedNames(t *testing.T) {
	top := t.TempDir()
	odd := filepath.Join(top, "..meta")
	if err := os.MkdirAll(odd, 0o755); err != nil {
		t.Fatal(err)
	}
	if rel, err := moduleRel(top, odd, top); err != nil || rel != "..meta" {
		t.Errorf("rel=%q err=%v", rel, err)
	}
}

func FuzzParseLog(f *testing.F) {
	f.Add("abc\x1f2026-03-10T12:00:00Z\x1fsubject\n")
	f.Add("")
	f.Add("broken\n")
	f.Add("a\x1fnot-a-date\x1fs\n")
	f.Fuzz(func(t *testing.T, in string) {
		commits, err := parseLog(in)
		if err != nil {
			return
		}
		for _, c := range commits {
			if _, perr := time.Parse(time.RFC3339, c.Date); perr != nil {
				t.Fatalf("accepted an unparsable date %q", c.Date)
			}
		}
	})
}
