package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseOverridesDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`
distance_threshold: 0.4
god_ce_threshold: 5
exported_only: true
exclude: ["**/mocks/**"]
check:
  max_pain_packages: 2
`))
	if err != nil {
		t.Fatal(err)
	}
	want := Default()
	want.DistanceThreshold = 0.4
	want.GodCeThreshold = 5
	want.ExportedOnly = true
	want.Exclude = []string{"**/mocks/**"}
	want.Check.MaxPainPackages = 2
	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestParseEmptyIsDefault(t *testing.T) {
	cfg, err := Parse(nil)
	if err != nil || !cmp.Equal(cfg, Default()) {
		t.Errorf("cfg=%+v err=%v", cfg, err)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct{ name, yaml, want string }{
		{"unknown key", "distance_treshold: 0.5\n", "distance_treshold"},
		{"range", "distance_threshold: 1.5\n", "distance_threshold"},
		{"negative", "check:\n  max_pain_packages: -1\n", "check.max_pain_packages"},
		{"wrong type reports the line", "god_ce_threshold: many\n", "line 1"},
		{"bad yaml", "a: [\n", "parsing config"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want mention of %q", err, tt.want)
			}
		})
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	if cfg, err := Discover(dir, ""); err != nil || !cmp.Equal(cfg, Default()) {
		t.Fatalf("missing file: cfg=%+v err=%v", cfg, err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("god_ce_threshold: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Discover(dir, "")
	if err != nil || cfg.GodCeThreshold != 3 {
		t.Fatalf("discovered: cfg=%+v err=%v", cfg, err)
	}
	if _, err := Discover(dir, filepath.Join(dir, "nope.yaml")); err == nil {
		t.Error("explicit missing file must fail")
	}
}

func TestIsExcluded(t *testing.T) {
	c := Config{Exclude: []string{"**/mocks/**"}}
	if !c.IsExcluded("m/mocks/x") || c.IsExcluded("m/x") {
		t.Error("exclude matching wrong")
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"", "distance_threshold: 0.5\n", "check:\n  fail_on_cycles: false\n", "a: [", "exclude: 3\n", "- x\n"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		cfg, err := Parse(data)
		if err == nil {
			if verr := cfg.Validate(); verr != nil {
				t.Fatalf("Parse accepted an invalid config: %v", verr)
			}
		}
	})
}

func TestParseIgnore(t *testing.T) {
	cfg, err := Parse([]byte("ignore:\n  - rule: pain-zone\n    package: '**/internal/config'\n    reason: value struct read once at startup\n"))
	if err != nil {
		t.Fatal(err)
	}
	reason, ok := cfg.IgnoreReason("pain-zone", "m/internal/config")
	if !ok || reason != "value struct read once at startup" {
		t.Errorf("IgnoreReason = %q, %v", reason, ok)
	}
	if _, ok := cfg.IgnoreReason("god-package", "m/internal/config"); ok {
		t.Error("other rules must not be silenced")
	}
	if _, ok := cfg.IgnoreReason("pain-zone", "m/internal/other"); ok {
		t.Error("other packages must not be silenced")
	}
}

func TestParseIgnoreRejectsIncompleteEntries(t *testing.T) {
	for name, in := range map[string]string{
		"unknown rule":   "ignore:\n  - rule: pain\n    package: x\n    reason: r\n",
		"missing reason": "ignore:\n  - rule: pain-zone\n    package: x\n",
		"blank reason":   "ignore:\n  - rule: pain-zone\n    package: x\n    reason: '  '\n",
		"missing glob":   "ignore:\n  - rule: pain-zone\n    reason: r\n",
		"unknown key":    "ignore:\n  - rule: pain-zone\n    package: x\n    reason: r\n    why: r\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(in)); err == nil {
				t.Error("expected an error")
			}
		})
	}
}
