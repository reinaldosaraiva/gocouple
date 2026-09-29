package check

import (
	"fmt"
	"sort"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// Rule names reported in violations.
const (
	RuleMaxDistance  = "max-distance"
	RulePainPackages = "max-pain-packages"
	RuleCycles       = "fail-on-cycles"
	RuleSeverity     = "fail-on"
	RuleRegression   = "baseline"
)

// Violation is one failed check with the values that failed it.
type Violation struct {
	Rule      string
	Package   string
	Message   string
	Severity  string
	Threshold string
	Actual    string
}

// Options are the effective check settings.
type Options struct {
	MaxDistance     float64
	MinCaForPain    int
	MaxPainPackages int
	FailOnCycles    bool
	FailOn          string
	Tolerance       float64
}

// FromConfig builds Options from the configuration's check block.
func FromConfig(c config.Config) Options {
	return Options{
		MaxDistance:     c.Check.MaxDistance,
		MinCaForPain:    c.MinCaForPain,
		MaxPainPackages: c.Check.MaxPainPackages,
		FailOnCycles:    c.Check.FailOnCycles,
		FailOn:          model.SeverityError,
		Tolerance:       0.02,
	}
}

var severityRank = map[string]int{model.SeverityInfo: 1, model.SeverityWarning: 2, model.SeverityError: 3}

// ValidSeverity reports whether s is a known severity name.
func ValidSeverity(s string) bool {
	_, ok := severityRank[s]
	return ok
}

// Evaluate applies the thresholds to current. max-distance is a per-package
// ceiling on packages that matter: those in the uselessness zone and those in
// the pain zone with at least MinCaForPain dependents. With a non-nil baseline only
// regressions relative to it fail, and absolute thresholds are ignored.
func Evaluate(current, baseline *model.Snapshot, opts Options) []Violation {
	if baseline != nil {
		return regressions(current, baseline, opts)
	}
	var out []Violation
	for _, p := range current.Packages {
		d := float64(p.Distance)
		relevant := p.Zone == model.ZoneUselessness || (p.Zone == model.ZonePain && p.Ca >= opts.MinCaForPain)
		if relevant && d > opts.MaxDistance {
			out = append(out, Violation{
				Rule: RuleMaxDistance, Package: p.Path, Severity: model.SeverityError,
				Message:   fmt.Sprintf("%s: distance %.4f exceeds %.4f (zone %s, Ca %d)", p.Path, d, opts.MaxDistance, p.Zone, p.Ca),
				Threshold: fmt.Sprintf("%.4f", opts.MaxDistance), Actual: fmt.Sprintf("%.4f", d),
			})
		}
	}
	pain := painDiagnostics(current)
	if len(pain) > opts.MaxPainPackages {
		out = append(out, Violation{
			Rule: RulePainPackages, Severity: model.SeverityError,
			Message:   fmt.Sprintf("%d packages in the pain zone (allowed %d): %s", len(pain), opts.MaxPainPackages, joinPackages(pain)),
			Threshold: fmt.Sprint(opts.MaxPainPackages), Actual: fmt.Sprint(len(pain)),
		})
	}
	if opts.FailOnCycles {
		for _, d := range current.Diagnostics {
			if d.ID == "dependency-cycle" {
				out = append(out, Violation{
					Rule: RuleCycles, Package: d.Package, Severity: model.SeverityError,
					Message: d.Message, Threshold: "0", Actual: "1",
				})
			}
		}
	}
	if rank, ok := severityRank[opts.FailOn]; ok {
		for _, d := range current.Diagnostics {
			if d.ID == "dependency-cycle" {
				continue
			}
			if severityRank[d.Severity] >= rank {
				out = append(out, Violation{
					Rule: RuleSeverity, Package: d.Package, Severity: d.Severity,
					Message: fmt.Sprintf("%s: %s", d.ID, d.Message), Threshold: opts.FailOn, Actual: d.Severity,
				})
			}
		}
	}
	return out
}

func regressions(current, baseline *model.Snapshot, opts Options) []Violation {
	var out []Violation
	before := map[string]bool{}
	for _, d := range baseline.Diagnostics {
		if d.ID == "pain-zone" {
			before[d.Package] = true
		}
	}
	for _, d := range painDiagnostics(current) {
		if !before[d.Package] {
			out = append(out, Violation{
				Rule: RuleRegression, Package: d.Package, Severity: model.SeverityError,
				Message:   fmt.Sprintf("new package in the pain zone: %s", d.Package),
				Threshold: "not in baseline", Actual: "pain-zone",
			})
		}
	}
	oldCycles := map[string]bool{}
	for _, c := range baseline.Cycles {
		oldCycles[cycleKey(c)] = true
	}
	for _, c := range current.Cycles {
		if opts.FailOnCycles && !oldCycles[cycleKey(c)] {
			out = append(out, Violation{
				Rule: RuleRegression, Package: c[0], Severity: model.SeverityError,
				Message:   fmt.Sprintf("new dependency cycle: %s", cycleKey(c)),
				Threshold: "not in baseline", Actual: "cycle",
			})
		}
	}
	delta := float64(current.Summary.AvgDistance) - float64(baseline.Summary.AvgDistance)
	if delta > opts.Tolerance {
		out = append(out, Violation{
			Rule: RuleRegression, Severity: model.SeverityError,
			Message: fmt.Sprintf("average distance rose by %.4f (baseline %.4f, now %.4f, tolerance %.4f)",
				delta, float64(baseline.Summary.AvgDistance), float64(current.Summary.AvgDistance), opts.Tolerance),
			Threshold: fmt.Sprintf("+%.4f", opts.Tolerance), Actual: fmt.Sprintf("%+.4f", delta),
		})
	}
	return out
}

func painDiagnostics(s *model.Snapshot) []model.Diagnostic {
	var out []model.Diagnostic
	for _, d := range s.Diagnostics {
		if d.ID == "pain-zone" {
			out = append(out, d)
		}
	}
	return out
}

func joinPackages(ds []model.Diagnostic) string {
	names := make([]string, len(ds))
	for i, d := range ds {
		names[i] = d.Package
	}
	sort.Strings(names)
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

func cycleKey(members []string) string {
	sorted := append([]string(nil), members...)
	sort.Strings(sorted)
	out := ""
	for i, m := range sorted {
		if i > 0 {
			out += " -> "
		}
		out += m
	}
	return out
}
