package analysis

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

func generatedDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "generated"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func diagKeys(ds []model.Diagnostic) []string {
	out := []string{}
	for _, d := range ds {
		out = append(out, d.ID+" "+filepath.Base(d.Package))
	}
	return out
}

func TestGeneratedCodeIsNotDiagnosed(t *testing.T) {
	snap, err := Run(t.Context(), Options{Dir: generatedDir(t), Config: config.Default()})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"pain-zone settings"}, diagKeys(snap.Diagnostics)); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
	got := []string{}
	for _, s := range snap.Suppressed {
		got = append(got, s.ID+" "+filepath.Base(s.Package)+": "+s.Reason)
	}
	if diff := cmp.Diff([]string{"pain-zone gen: generated code"}, got); diff != "" {
		t.Errorf("suppressed (-want +got):\n%s", diff)
	}
	generated := map[string]bool{}
	for _, p := range snap.Packages {
		generated[filepath.Base(p.Path)] = p.Generated
	}
	want := map[string]bool{"gen": true, "mixed": false, "settings": false, "a": false, "b": false, "c": false}
	if diff := cmp.Diff(want, generated); diff != "" {
		t.Errorf("generated flags (-want +got):\n%s", diff)
	}
}

func TestIgnoreRuleSilencesWithReason(t *testing.T) {
	cfg := config.Default()
	cfg.Ignore = []config.IgnoreRule{{Rule: "pain-zone", Package: "**/settings", Reason: "startup value struct"}}
	snap, err := Run(t.Context(), Options{Dir: generatedDir(t), Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Diagnostics) != 0 {
		t.Errorf("diagnostics = %v, want none", diagKeys(snap.Diagnostics))
	}
	reasons := map[string]string{}
	for _, s := range snap.Suppressed {
		reasons[filepath.Base(s.Package)] = s.Reason
	}
	want := map[string]string{"gen": "generated code", "settings": "startup value struct"}
	if diff := cmp.Diff(want, reasons); diff != "" {
		t.Errorf("reasons (-want +got):\n%s", diff)
	}
	if snap.Summary.Packages != 6 {
		t.Errorf("ignored packages must stay measured, got %d packages", snap.Summary.Packages)
	}
}

func TestUnusedIgnoreIsWarned(t *testing.T) {
	cfg := config.Default()
	cfg.Ignore = []config.IgnoreRule{
		{Rule: "pain-zone", Package: "**/settings", Reason: "startup value struct"},
		{Rule: "god-package", Package: "**/nothing", Reason: "stale entry"},
	}
	snap, err := Run(t.Context(), Options{Dir: generatedDir(t), Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ignore[1]: god-package on **/nothing silenced no finding (stale entry)"}
	if diff := cmp.Diff(want, snap.Warnings); diff != "" {
		t.Errorf("warnings (-want +got):\n%s", diff)
	}
	if len(snap.Diagnostics) != 0 {
		t.Errorf("a stale ignore must not change the diagnostics, got %v", diagKeys(snap.Diagnostics))
	}
}

func TestNoIgnoreNoWarnings(t *testing.T) {
	snap, err := Run(t.Context(), Options{Dir: generatedDir(t), Config: config.Default()})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Warnings != nil {
		t.Errorf("warnings = %v, want nil so the JSON omits the field", snap.Warnings)
	}
}
