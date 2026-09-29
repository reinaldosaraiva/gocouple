package analysis

import (
	"path/filepath"
	"testing"

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
