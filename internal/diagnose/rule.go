package diagnose

import (
	"context"
	"sort"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// Rule inspects a snapshot and reports architectural findings.
type Rule interface {
	Check(ctx context.Context, snap *model.Snapshot, cfg config.Config) []model.Diagnostic
}

// Rules returns the registered rules in a fixed order. Adding a rule means
// adding one file and one entry here.
func Rules() []Rule {
	return []Rule{
		painZone{},
		uselessnessZone{},
		godPackage{},
		dependencyCycle{},
		concreteHotspot{},
		sdpViolation{},
		wastedAbstraction{},
	}
}

// Run applies every rule and returns diagnostics sorted by package, id and
// message. The result is never nil.
func Run(ctx context.Context, snap *model.Snapshot, cfg config.Config, rules []Rule) []model.Diagnostic {
	out := []model.Diagnostic{}
	for _, r := range rules {
		out = append(out, r.Check(ctx, snap, cfg)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Message < b.Message
	})
	return out
}
