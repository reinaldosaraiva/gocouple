package analysis

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/reinaldosaraiva/gocouple/internal/config"
)

func TestRunOnFixtures(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "orders-legacy"))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Run(t.Context(), Options{Dir: dir, Config: config.Default(), ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Module != "example.com/orderslegacy" || snap.ToolVersion != "test" || snap.Summary.Packages != 12 {
		t.Errorf("snapshot = %s %s %+v", snap.Module, snap.ToolVersion, snap.Summary)
	}
	if len(snap.Diagnostics) != 8 || len(snap.Interfaces) == 0 {
		t.Errorf("diagnostics=%d interfaces=%d", len(snap.Diagnostics), len(snap.Interfaces))
	}
}

func TestRunAppliesExcludeAndReportsErrors(t *testing.T) {
	dir, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "orders-legacy"))
	cfg := config.Default()
	cfg.Exclude = []string{"**/internal/logger", "**/internal/contracts"}
	snap, err := Run(t.Context(), Options{Dir: dir, Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Summary.Packages != 10 {
		t.Errorf("packages = %d, want 10", snap.Summary.Packages)
	}
	if _, err := Run(t.Context(), Options{Dir: t.TempDir(), Config: cfg}); err == nil {
		t.Error("expected error for a directory without a module")
	}
}

func benchDir(b *testing.B, rel ...string) string {
	b.Helper()
	dir, err := filepath.Abs(filepath.Join(rel...))
	if err != nil {
		b.Fatal(err)
	}
	return dir
}

func BenchmarkRunOrdersLegacy(b *testing.B) {
	opts := Options{Dir: benchDir(b, "..", "..", "testdata", "orders-legacy"), Config: config.Default()}
	for b.Loop() {
		if _, err := Run(b.Context(), opts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRunSelf(b *testing.B) {
	opts := Options{Dir: benchDir(b, "..", ".."), Patterns: []string{"./..."}, Config: config.Default()}
	for b.Loop() {
		if _, err := Run(b.Context(), opts); err != nil {
			b.Fatal(err)
		}
	}
}

type fakeGit struct {
	shallow string
	log     string
	logErr  error
	calls   int
}

func (f *fakeGit) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	f.calls++
	switch {
	case args[0] == "rev-parse" && args[1] == "--is-inside-work-tree":
		return []byte("true\n"), nil
	case args[0] == "rev-parse":
		return []byte(f.shallow + "\n"), nil
	}
	return []byte(f.log), f.logErr
}

func simpleDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "simple"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func volatilityOptions(t *testing.T, git *fakeGit) Options {
	t.Helper()
	cfg := config.Default()
	cfg.VolatilitySince = "180d"
	return Options{
		Dir: simpleDir(t), Config: cfg, Git: git,
		Now: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
	}
}

func TestRunComputesChurnAndNormalizedVolatility(t *testing.T) {
	git := &fakeGit{shallow: "false", log: "\x1eaaa\n\na/a.go\nb/b.go\n\x1ebbb\n\na/a.go\n"}
	snap, err := Run(t.Context(), volatilityOptions(t, git))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Config.VolatilitySince != "2026-04-03T12:00:00Z" {
		t.Errorf("since = %q", snap.Config.VolatilitySince)
	}
	got := map[string][2]float64{}
	for _, p := range snap.Packages {
		got[p.Path] = [2]float64{float64(p.Churn), float64(p.Volatility)}
	}
	want := map[string][2]float64{
		"example.com/simple/a": {2, 1},
		"example.com/simple/b": {1, 0.5},
		"example.com/simple/c": {0, 0},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("churn (-want +got):\n%s", diff)
	}
}

func TestRunShallowRepositoryWarnsWithoutVolatility(t *testing.T) {
	git := &fakeGit{shallow: "true", log: "\x1eaaa\n\na/a.go\n"}
	snap, err := Run(t.Context(), volatilityOptions(t, git))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Config.VolatilitySince != "" || len(snap.Warnings) != 1 {
		t.Errorf("since=%q warnings=%v", snap.Config.VolatilitySince, snap.Warnings)
	}
	for _, p := range snap.Packages {
		if p.Churn != 0 || p.Volatility != 0 {
			t.Errorf("%s carries volatility without a measurement", p.Path)
		}
	}
}

func TestRunLogFailureAndInvalidWindowAreErrors(t *testing.T) {
	if _, err := Run(t.Context(), volatilityOptions(t, &fakeGit{shallow: "false", logErr: errors.New("boom")})); err == nil {
		t.Error("a failing git log must fail the analysis")
	}
	opts := volatilityOptions(t, &fakeGit{shallow: "false"})
	opts.Config.VolatilitySince = "soon"
	if _, err := Run(t.Context(), opts); err == nil {
		t.Error("an invalid window must fail the analysis")
	}
}

func TestRunWithoutVolatilityNeverCallsGit(t *testing.T) {
	git := &fakeGit{shallow: "false"}
	opts := volatilityOptions(t, git)
	opts.Config.VolatilitySince = ""
	if _, err := Run(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	if git.calls != 0 {
		t.Errorf("git called %d times with volatility off", git.calls)
	}
}
