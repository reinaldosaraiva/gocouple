package diagnose

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

func ids(ds []model.Diagnostic) []string {
	out := []string{}
	for _, d := range ds {
		out = append(out, d.ID+":"+d.Package)
	}
	return out
}

func run(snap *model.Snapshot, cfg config.Config) []model.Diagnostic {
	return Run(t0(), snap, cfg, Rules())
}

func TestPainZoneThreshold(t *testing.T) {
	cfg := config.Default()
	tests := []struct {
		name string
		ca   int
		zone string
		want []string
	}{
		{"below min ca", cfg.MinCaForPain - 1, model.ZonePain, []string{}},
		{"at min ca", cfg.MinCaForPain, model.ZonePain, []string{"pain-zone:m/a"}},
		{"not in pain", 10, model.ZoneMainSequence, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := &model.Snapshot{Packages: []model.Package{{Path: "m/a", Ca: tt.ca, Zone: tt.zone, Abstractness: 0.5}}}
			if diff := cmp.Diff(tt.want, ids(run(snap, cfg))); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestGodPackageThreshold(t *testing.T) {
	cfg := config.Default()
	for _, tt := range []struct {
		ce   int
		want []string
	}{{cfg.GodCeThreshold - 1, []string{}}, {cfg.GodCeThreshold, []string{"god-package:m/a"}}} {
		snap := &model.Snapshot{Packages: []model.Package{{Path: "m/a", Ce: tt.ce, Zone: model.ZoneMainSequence}}}
		if diff := cmp.Diff(tt.want, ids(run(snap, cfg))); diff != "" {
			t.Errorf("ce=%d (-want +got):\n%s", tt.ce, diff)
		}
	}
}

func TestConcreteHotspot(t *testing.T) {
	cfg := config.Default()
	tests := []struct {
		name string
		ca   int
		a    model.Ratio
		want []string
	}{
		{"below", cfg.HotspotCaThreshold - 1, 0, []string{}},
		{"at threshold", cfg.HotspotCaThreshold, 0, []string{"concrete-hotspot:m/a"}},
		{"abstract enough", cfg.HotspotCaThreshold, 0.1, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := &model.Snapshot{Packages: []model.Package{{Path: "m/a", Ca: tt.ca, Abstractness: tt.a, Zone: model.ZoneMainSequence}}}
			if diff := cmp.Diff(tt.want, ids(run(snap, cfg))); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestUselessnessZone(t *testing.T) {
	snap := &model.Snapshot{Packages: []model.Package{
		{Path: "m/a", Zone: model.ZoneUselessness},
		{Path: "m/b", Zone: model.ZoneMainSequence},
	}}
	if diff := cmp.Diff([]string{"uselessness-zone:m/a"}, ids(run(snap, config.Default()))); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestDependencyCycleEvidence(t *testing.T) {
	snap := &model.Snapshot{
		Packages: []model.Package{
			{Path: "m/a", Imports: []string{"m/b", "m/c"}, Zone: model.ZoneMainSequence},
			{Path: "m/b", Imports: []string{"m/c"}, Zone: model.ZoneMainSequence},
			{Path: "m/c", Imports: []string{"m/a"}, Zone: model.ZoneMainSequence},
		},
		Cycles: [][]string{{"m/a", "m/b", "m/c"}},
	}
	got := run(snap, config.Default())
	if len(got) != 1 || got[0].ID != "dependency-cycle" || got[0].Severity != model.SeverityError {
		t.Fatalf("diagnostics = %+v", got)
	}
	if diff := cmp.Diff([]string{"m/a", "m/c"}, got[0].Evidence["cycle"]); diff != "" {
		t.Errorf("shortest cycle (-want +got):\n%s", diff)
	}
	if got[0].Message != "import cycle: m/a -> m/c -> m/a" {
		t.Errorf("message = %q", got[0].Message)
	}
}

func TestSDPViolation(t *testing.T) {
	pkgs := func(iP, iQ model.Ratio, pPath string) []model.Package {
		return []model.Package{
			{Path: pPath, Imports: []string{"m/q"}, Instability: iP, Zone: model.ZoneMainSequence},
			{Path: "m/q", Instability: iQ, Zone: model.ZoneMainSequence},
		}
	}
	cfg := config.Default()
	tests := []struct {
		name string
		snap *model.Snapshot
		cfg  config.Config
		want []string
	}{
		{"equal instability ok", &model.Snapshot{Packages: pkgs(0.5, 0.5, "m/p")}, cfg, []string{}},
		{"stable depends on unstable", &model.Snapshot{Packages: pkgs(0.2, 0.8, "m/p")}, cfg, []string{"sdp-violation:m/p"}},
		{"within tolerance", &model.Snapshot{Packages: pkgs(0.5, 0.6, "m/p")}, config.Config{SDPTolerance: 0.1}, []string{}},
		{"composition root exempt", &model.Snapshot{Packages: pkgs(0.2, 0.8, "m/cmd/app")}, cfg, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, ids(Run(t0(), tt.snap, tt.cfg, []Rule{sdpViolation{}}))); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestRunSortedAndNonNil(t *testing.T) {
	if got := run(&model.Snapshot{}, config.Default()); got == nil || len(got) != 0 {
		t.Errorf("empty run = %#v", got)
	}
	snap := &model.Snapshot{Packages: []model.Package{
		{Path: "m/b", Ce: 20, Zone: model.ZoneUselessness},
		{Path: "m/a", Ce: 20, Zone: model.ZoneMainSequence},
	}}
	want := []string{"god-package:m/a", "god-package:m/b", "uselessness-zone:m/b"}
	if diff := cmp.Diff(want, ids(run(snap, config.Default()))); diff != "" {
		t.Errorf("order (-want +got):\n%s", diff)
	}
}

func t0() context.Context { return context.Background() }

func TestDependencyCycleSkipsTrivialComponent(t *testing.T) {
	snap := &model.Snapshot{Cycles: [][]string{{"m/a"}}}
	if got := run(snap, config.Default()); len(got) != 0 {
		t.Errorf("diagnostics = %+v", got)
	}
}

func TestWastedAbstraction(t *testing.T) {
	tests := []struct {
		name  string
		usage model.InterfaceUsage
		want  []string
	}{
		{"consumers only", model.InterfaceUsage{Package: "m/c", Name: "I", IfaceRefs: []string{"m/a"}}, []string{}},
		{"no consumers at all", model.InterfaceUsage{Package: "m/c", Name: "I"}, []string{"wasted-abstraction:m/c"}},
		{"not inverted takes precedence", model.InterfaceUsage{Package: "m/c", Name: "I", ConcreteRefs: []string{"m/a"}}, []string{"wasted-abstraction:m/c"}},
		{"equal refs is inverted enough", model.InterfaceUsage{Package: "m/c", Name: "I", IfaceRefs: []string{"m/a"}, ConcreteRefs: []string{"m/b"}}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := &model.Snapshot{Interfaces: []model.InterfaceUsage{tt.usage}}
			got := Run(t0(), snap, config.Default(), []Rule{wastedAbstraction{}})
			if diff := cmp.Diff(tt.want, ids(got)); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
	snap := &model.Snapshot{Interfaces: []model.InterfaceUsage{{Package: "m/c", Name: "I", ConcreteRefs: []string{"m/a"}}}}
	d := Run(t0(), snap, config.Default(), []Rule{wastedAbstraction{}})[0]
	if d.Evidence["concrete_refs"] != 1 || d.Evidence["iface_refs"] != 0 {
		t.Errorf("evidence = %v", d.Evidence)
	}
}

func TestSuppressKeepsCyclesThroughGeneratedPackages(t *testing.T) {
	snap := &model.Snapshot{Packages: []model.Package{{Path: "m/gen", Generated: true}, {Path: "m/a"}}}
	diags := []model.Diagnostic{
		{ID: "dependency-cycle", Package: "m/gen"},
		{ID: "pain-zone", Package: "m/gen"},
		{ID: "pain-zone", Package: "m/a"},
	}
	kept, suppressed := Suppress(snap, config.Default(), diags)
	if diff := cmp.Diff([]string{"dependency-cycle:m/gen", "pain-zone:m/a"}, ids(kept)); diff != "" {
		t.Errorf("kept (-want +got):\n%s", diff)
	}
	if len(suppressed) != 1 || suppressed[0].ID != "pain-zone" || suppressed[0].Reason != "generated code" {
		t.Errorf("suppressed = %+v", suppressed)
	}
}
