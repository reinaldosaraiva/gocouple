package volatility

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reinaldosaraiva/gocouple/internal/gitrun"

	"github.com/google/go-cmp/cmp"
)

type call struct {
	Dir  string
	Args []string
}

type fakeRunner struct {
	calls   []call
	outputs map[string]string
	errs    map[string]error
}

func (f *fakeRunner) Run(_ context.Context, dir string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, call{dir, args})
	key := strings.Join(args[:2], " ")
	if err := f.errs[key]; err != nil {
		return nil, err
	}
	return []byte(f.outputs[key]), nil
}

func newFake(logOut string) *fakeRunner {
	return &fakeRunner{
		outputs: map[string]string{
			"rev-parse --is-inside-work-tree":            "true\n",
			"rev-parse --is-shallow-repository":          "false\n",
			"log --since-as-filter=2026-03-01T00:00:00Z": logOut,
		},
		errs: map[string]error{},
	}
}

var since = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

var pkgs = []string{"m", "m/a", "m/a/sub", "m/b", "m/c"}

func TestComputeParsesLog(t *testing.T) {
	logOut := "\x1eaaa\n\na/a.go\na/b.go\nb/b.go\n" +
		"\x1ebbb\n\na/a.go\na/sub/s.go\nREADME.md\n" +
		"\x1eccc\n\na/a_test.go\n" +
		"\x1eddd\n\n" +
		"\x1eeee\n\nmain.go\nvendor/x/x.go\n"
	fake := newFake(logOut)
	res, err := Compute(t.Context(), fake, "/repo/mod", "m", pkgs, since)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"m": 1, "m/a": 2, "m/a/sub": 1, "m/b": 1, "m/c": 0}
	if diff := cmp.Diff(want, res.Churn); diff != "" {
		t.Errorf("churn (-want +got):\n%s", diff)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %v", res.Warnings)
	}
}

func TestComputeQuotedPaths(t *testing.T) {
	logOut := "\x1eaaa\n\n\"caf\\303\\251/x.go\"\n\"we\\\"ird/y.go\"\n\" sp/z.go\"\n\"caf\\303\\251/x_test.go\"\n"
	res, err := Compute(t.Context(), newFake(logOut), "/repo/mod", "m", []string{"m/café", "m/we\"ird", "m/ sp", "m/x"}, since)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"m/café": 1, "m/we\"ird": 1, "m/ sp": 1, "m/x": 0}
	if diff := cmp.Diff(want, res.Churn); diff != "" {
		t.Errorf("churn (-want +got):\n%s", diff)
	}
}

func TestComputeCommandLine(t *testing.T) {
	fake := newFake("")
	if _, err := Compute(t.Context(), fake, "/repo/mod", "m", pkgs, since); err != nil {
		t.Fatal(err)
	}
	want := []call{
		{"/repo/mod", []string{"rev-parse", "--is-inside-work-tree"}},
		{"/repo/mod", []string{"rev-parse", "--is-shallow-repository"}},
		{"/repo/mod", []string{"log", "--since-as-filter=2026-03-01T00:00:00Z", "--no-merges", "--name-only", "--relative", "--format=%x1e%H", "--", "."}},
	}
	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Errorf("calls (-want +got):\n%s", diff)
	}
}

func TestComputeEveryPackagePresent(t *testing.T) {
	res, err := Compute(t.Context(), newFake(""), "/repo/mod", "m", pkgs, since)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pkgs {
		if n, ok := res.Churn[p]; !ok || n != 0 {
			t.Errorf("churn[%s] = %d, present %v", p, n, ok)
		}
	}
}

func TestComputeEnvironmentWarnings(t *testing.T) {
	tests := map[string]func(*fakeRunner){
		"not a work tree": func(f *fakeRunner) { f.errs["rev-parse --is-inside-work-tree"] = errors.New("not a git repository") },
		"outside tree":    func(f *fakeRunner) { f.outputs["rev-parse --is-inside-work-tree"] = "false\n" },
		"shallow probe":   func(f *fakeRunner) { f.errs["rev-parse --is-shallow-repository"] = errors.New("old git") },
		"shallow":         func(f *fakeRunner) { f.outputs["rev-parse --is-shallow-repository"] = "true\n" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fake := newFake("\x1eaaa\n\na/a.go\n")
			mutate(fake)
			res, err := Compute(t.Context(), fake, "/repo/mod", "m", pkgs, since)
			if err != nil {
				t.Fatalf("Compute: %v", err)
			}
			if res.Churn != nil || len(res.Warnings) != 1 {
				t.Errorf("churn = %v, warnings = %v", res.Churn, res.Warnings)
			}
			for _, c := range fake.calls {
				if c.Args[0] == "log" {
					t.Errorf("log ran despite %s", name)
				}
			}
		})
	}
}

func TestComputeLogFailureIsError(t *testing.T) {
	fake := newFake("")
	fake.errs["log --since-as-filter=2026-03-01T00:00:00Z"] = errors.New("boom")
	if _, err := Compute(t.Context(), fake, "/repo/mod", "m", pkgs, since); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v", err)
	}
}

func TestParseSince(t *testing.T) {
	now := time.Date(2026, 9, 30, 15, 4, 5, 0, time.UTC)
	tests := []struct {
		in      string
		want    time.Time
		wantErr bool
	}{
		{"180d", time.Date(2026, 4, 3, 15, 4, 5, 0, time.UTC), false},
		{"0d", now, false},
		{"2026-01-01", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), false},
		{"2026-01-01T10:00:00-03:00", time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC), false},
		{" 30d ", time.Date(2026, 8, 31, 15, 4, 5, 0, time.UTC), false},
		{"", time.Time{}, true},
		{"soon", time.Time{}, true},
		{"-5d", time.Time{}, true},
		{"2026-13-01", time.Time{}, true},
	}
	for _, tc := range tests {
		got, err := ParseSince(tc.in, now)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseSince(%q) error = %v", tc.in, err)
			continue
		}
		if !tc.wantErr && !got.Equal(tc.want) {
			t.Errorf("ParseSince(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func gitCmd(t *testing.T, dir, date string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func commit(t *testing.T, repo, date, msg string, files ...string) {
	t.Helper()
	for _, f := range files {
		path := filepath.Join(repo, f)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		prev, _ := os.ReadFile(path)
		if err := os.WriteFile(path, append(prev, []byte("// "+msg+"\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitCmd(t, repo, date, "add", "-A")
	gitCmd(t, repo, date, "commit", "-m", msg)
}

func TestComputeSyntheticRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	repo := t.TempDir()
	gitCmd(t, repo, "2026-01-01T12:00:00Z", "init", "-b", "main")
	mod := filepath.Join(repo, "mod")
	commit(t, repo, "2026-01-10T12:00:00Z", "old", "mod/go.mod", "mod/a/a.go", "mod/b/b.go", "mod/c/c.go", "outside/o.go")
	commit(t, repo, "2026-03-05T12:00:00Z", "a1", "mod/a/a.go")
	commit(t, repo, "2026-02-01T12:00:00Z", "b-out-of-order", "mod/b/b.go")
	commit(t, repo, "2026-03-10T12:00:00Z", "ab", "mod/a/a.go", "mod/a/extra.go", "mod/b/b.go")
	commit(t, repo, "2026-03-11T12:00:00Z", "test-only", "mod/c/c_test.go")
	commit(t, repo, "2026-03-12T12:00:00Z", "docs", "mod/c/README.md")
	commit(t, repo, "2026-03-13T12:00:00Z", "sub", "mod/a/sub/s.go")
	commit(t, repo, "2026-03-14T12:00:00Z", "outside", "outside/o.go")
	commit(t, repo, "2026-03-14T13:00:00Z", "unicode", "mod/café/c.go")
	gitCmd(t, repo, "2026-03-15T12:00:00Z", "checkout", "-b", "side", "HEAD~1")
	commit(t, repo, "2026-03-15T12:00:00Z", "side", "mod/c/c.go")
	gitCmd(t, repo, "2026-03-16T12:00:00Z", "checkout", "main")
	gitCmd(t, repo, "2026-03-16T12:00:00Z", "merge", "--no-ff", "-m", "merge", "side")

	pkgs := []string{"m", "m/a", "m/a/sub", "m/b", "m/c", "m/café"}
	res, err := Compute(t.Context(), gitrun.Exec{}, mod, "m", pkgs, since)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"m": 0, "m/a": 2, "m/a/sub": 1, "m/b": 1, "m/c": 1, "m/café": 1}
	if diff := cmp.Diff(want, res.Churn); diff != "" {
		t.Errorf("churn (-want +got):\n%s", diff)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %v", res.Warnings)
	}
}
