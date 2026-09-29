package check

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

func opts() Options { return FromConfig(config.Default()) }

func rules(vs []Violation) []string {
	out := []string{}
	for _, v := range vs {
		out = append(out, v.Rule+":"+v.Package)
	}
	return out
}

func snap(pkgs []model.Package, diags []model.Diagnostic, cycles [][]string, avg float64) *model.Snapshot {
	return &model.Snapshot{Packages: pkgs, Diagnostics: diags, Cycles: cycles, Summary: model.Summary{AvgDistance: model.Ratio(avg)}}
}

func pain(pkg string) model.Diagnostic {
	return model.Diagnostic{ID: "pain-zone", Severity: model.SeverityWarning, Package: pkg}
}

func TestMaxDistanceOnRelevantPackages(t *testing.T) {
	o := opts()
	tests := []struct {
		name string
		pkg  model.Package
		want []string
	}{
		{"pain with dependents above", model.Package{Path: "m/a", Zone: model.ZonePain, Ca: 3, Distance: 0.71}, []string{"max-distance:m/a"}},
		{"pain exactly at ceiling", model.Package{Path: "m/a", Zone: model.ZonePain, Ca: 3, Distance: 0.7}, []string{}},
		{"pain leaf is ignored", model.Package{Path: "m/a", Zone: model.ZonePain, Ca: 2, Distance: 1}, []string{}},
		{"uselessness above", model.Package{Path: "m/a", Zone: model.ZoneUselessness, Distance: 1}, []string{"max-distance:m/a"}},
		{"main sequence never", model.Package{Path: "m/a", Zone: model.ZoneMainSequence, Ca: 9, Distance: 0.5}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rules(Evaluate(snap([]model.Package{tt.pkg}, nil, nil, 0), nil, o))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestPainPackageCount(t *testing.T) {
	o := opts()
	one := snap(nil, []model.Diagnostic{pain("m/a")}, nil, 0)
	if diff := cmp.Diff([]string{"max-pain-packages:"}, rules(Evaluate(one, nil, o))); diff != "" {
		t.Errorf("default 0 allowed (-want +got):\n%s", diff)
	}
	o.MaxPainPackages = 1
	if got := Evaluate(one, nil, o); len(got) != 0 {
		t.Errorf("1 allowed: %v", got)
	}
	two := snap(nil, []model.Diagnostic{pain("m/a"), pain("m/b")}, nil, 0)
	if got := Evaluate(two, nil, o); len(got) != 1 || got[0].Actual != "2" {
		t.Errorf("2 pain packages: %+v", got)
	}
}

func TestCyclesAndSeverity(t *testing.T) {
	cycle := model.Diagnostic{ID: "dependency-cycle", Severity: model.SeverityError, Package: "m/a", Message: "cycle"}
	warn := model.Diagnostic{ID: "god-package", Severity: model.SeverityWarning, Package: "m/b", Message: "big"}
	s := snap(nil, []model.Diagnostic{cycle, warn}, nil, 0)

	o := opts()
	if diff := cmp.Diff([]string{"fail-on-cycles:m/a"}, rules(Evaluate(s, nil, o))); diff != "" {
		t.Errorf("default (-want +got):\n%s", diff)
	}
	o.FailOnCycles = false
	if got := Evaluate(s, nil, o); len(got) != 0 {
		t.Errorf("--fail-on-cycles=false must let cycles pass: %v", got)
	}
	o.FailOn = model.SeverityWarning
	if diff := cmp.Diff([]string{"fail-on:m/b"}, rules(Evaluate(s, nil, o))); diff != "" {
		t.Errorf("warning level (-want +got):\n%s", diff)
	}
	o.FailOn = model.SeverityError
	o.FailOnCycles = false
	clean := snap(nil, []model.Diagnostic{warn}, nil, 0)
	if got := Evaluate(clean, nil, o); len(got) != 0 {
		t.Errorf("warning below error level must pass: %v", got)
	}
}

func TestBaseline(t *testing.T) {
	o := opts()
	base := snap(nil, []model.Diagnostic{pain("m/old")}, [][]string{{"m/x", "m/y"}}, 0.40)
	tests := []struct {
		name string
		cur  *model.Snapshot
		want []string
	}{
		{"unchanged debt passes", snap(nil, []model.Diagnostic{pain("m/old")}, [][]string{{"m/y", "m/x"}}, 0.40), []string{}},
		{"improvement passes", snap(nil, nil, nil, 0.10), []string{}},
		{"new pain package fails", snap(nil, []model.Diagnostic{pain("m/old"), pain("m/new")}, [][]string{{"m/x", "m/y"}}, 0.40), []string{"baseline:m/new"}},
		{"new cycle fails", snap(nil, []model.Diagnostic{pain("m/old")}, [][]string{{"m/x", "m/y"}, {"m/p", "m/q"}}, 0.40), []string{"baseline:m/p"}},
		{"rise within tolerance passes", snap(nil, []model.Diagnostic{pain("m/old")}, [][]string{{"m/x", "m/y"}}, 0.42), []string{}},
		{"rise above tolerance fails", snap(nil, []model.Diagnostic{pain("m/old")}, [][]string{{"m/x", "m/y"}}, 0.4201), []string{"baseline:"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, rules(Evaluate(tt.cur, base, o))); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
	o.FailOnCycles = false
	if got := Evaluate(snap(nil, nil, [][]string{{"m/x", "m/y"}, {"m/p", "m/q"}}, 0.40), base, o); len(got) != 0 {
		t.Errorf("baseline mode must honor --fail-on-cycles=false: %v", got)
	}
	o.FailOnCycles = true
	absolute := snap([]model.Package{{Path: "m/a", Zone: model.ZoneUselessness, Distance: 1}}, nil, nil, 0.40)
	if got := Evaluate(absolute, base, o); len(got) != 0 {
		t.Errorf("baseline mode must ignore absolute thresholds: %v", got)
	}
}

func TestValidSeverity(t *testing.T) {
	for _, s := range []string{"info", "warning", "error"} {
		if !ValidSeverity(s) {
			t.Errorf("%q must be valid", s)
		}
	}
	if ValidSeverity("fatal") {
		t.Error("fatal must be invalid")
	}
}
