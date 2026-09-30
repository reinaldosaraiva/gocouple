package diagnose

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

func painSnapshot(since string, pkgs ...model.Package) *model.Snapshot {
	for i := range pkgs {
		pkgs[i].Ca = 5
		pkgs[i].Zone = model.ZonePain
	}
	return &model.Snapshot{Config: model.Config{VolatilitySince: since}, Packages: pkgs}
}

func TestDormantPainZoneIsSuppressedWithReason(t *testing.T) {
	cfg := config.Default()
	snap := painSnapshot("2026-03-01T00:00:00Z",
		model.Package{Path: "m/cold"},
		model.Package{Path: "m/hot", Churn: 4},
	)
	kept, suppressed := Suppress(snap, cfg, painZone{}.Check(t0(), snap, cfg))
	if diff := cmp.Diff([]string{"pain-zone:m/hot"}, ids(kept)); diff != "" {
		t.Errorf("kept (-want +got):\n%s", diff)
	}
	want := []model.Suppressed{{ID: "pain-zone", Package: "m/cold", Reason: "stable in window (no commit since 2026-03-01)"}}
	if len(suppressed) != 1 || suppressed[0].ID != want[0].ID || suppressed[0].Package != want[0].Package || suppressed[0].Reason != want[0].Reason {
		t.Errorf("suppressed = %+v, want %+v", suppressed, want)
	}
	if got := kept[0].Message; got[len(got)-len("changed 4 times since 2026-03-01"):] != "changed 4 times since 2026-03-01" {
		t.Errorf("message lacks churn: %s", got)
	}
	if kept[0].Evidence["churn"] != 4 {
		t.Errorf("evidence = %v", kept[0].Evidence)
	}
}

func TestPainZoneWithoutVolatilityIsUntouched(t *testing.T) {
	cfg := config.Default()
	snap := painSnapshot("", model.Package{Path: "m/cold"})
	kept, suppressed := Suppress(snap, cfg, painZone{}.Check(t0(), snap, cfg))
	if len(kept) != 1 || len(suppressed) != 0 {
		t.Errorf("kept=%v suppressed=%v", ids(kept), suppressed)
	}
	if _, ok := kept[0].Evidence["churn"]; ok {
		t.Errorf("churn evidence without volatility: %v", kept[0].Evidence)
	}
}

func TestDormantSuppressionOnlyAppliesToPainZone(t *testing.T) {
	snap := &model.Snapshot{Config: model.Config{VolatilitySince: "2026-03-01T00:00:00Z"}}
	diags := []model.Diagnostic{{ID: "god-package", Package: "m/cold"}, {ID: "dependency-cycle", Package: "m/cold"}}
	snap.Packages = []model.Package{{Path: "m/cold"}}
	kept, suppressed := Suppress(snap, config.Default(), diags)
	if len(kept) != 2 || len(suppressed) != 0 {
		t.Errorf("kept=%v suppressed=%v", ids(kept), suppressed)
	}
}

func TestDormantSuppressionKeepsIgnoreAndGeneratedPrecedence(t *testing.T) {
	cfg := config.Default()
	cfg.Ignore = []config.IgnoreRule{{Rule: "pain-zone", Package: "m/ign", Reason: "accepted by owner"}}
	snap := painSnapshot("2026-03-01T00:00:00Z",
		model.Package{Path: "m/ign"},
		model.Package{Path: "m/gen", Generated: true},
		model.Package{Path: "m/cold"},
	)
	_, suppressed := Suppress(snap, cfg, painZone{}.Check(t0(), snap, cfg))
	got := map[string]string{}
	for _, s := range suppressed {
		got[s.Package] = s.Reason
	}
	want := map[string]string{
		"m/ign":  "accepted by owner",
		"m/gen":  "generated code",
		"m/cold": "stable in window (no commit since 2026-03-01)",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("reasons (-want +got):\n%s", diff)
	}
}

func TestDormantSuppressionIgnoresMalformedBoundary(t *testing.T) {
	cfg := config.Default()
	snap := painSnapshot("not-a-date", model.Package{Path: "m/cold"})
	kept, suppressed := Suppress(snap, cfg, painZone{}.Check(t0(), snap, cfg))
	if len(kept) != 1 || len(suppressed) != 0 {
		t.Errorf("kept=%v suppressed=%v", ids(kept), suppressed)
	}
}

func TestPackageAbsentFromSnapshotIsNeverSuppressedAsDormant(t *testing.T) {
	snap := painSnapshot("2026-03-01T00:00:00Z", model.Package{Path: "m/known"})
	diags := []model.Diagnostic{{ID: "pain-zone", Package: "m/unknown"}}
	kept, suppressed := Suppress(snap, config.Default(), diags)
	if len(kept) != 1 || len(suppressed) != 0 {
		t.Errorf("kept=%v suppressed=%v", ids(kept), suppressed)
	}
}
