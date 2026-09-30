package diagnose

import (
	"fmt"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

const reasonGenerated = "generated code"

// Suppress splits diags into those to report and those silenced because the
// package holds only generated code or an ignore rule names it. Import cycles
// are never silenced for being generated: they are real wherever they start. The relative
// order of diags is kept in both results and neither is nil.
func Suppress(snap *model.Snapshot, cfg config.Config, diags []model.Diagnostic) ([]model.Diagnostic, []model.Suppressed) {
	generated := map[string]bool{}
	for _, p := range snap.Packages {
		if p.Generated {
			generated[p.Path] = true
		}
	}
	kept := []model.Diagnostic{}
	suppressed := []model.Suppressed{}
	for _, d := range diags {
		reason, ok := cfg.IgnoreReason(d.ID, d.Package)
		if !ok && generated[d.Package] && d.ID != "dependency-cycle" {
			reason, ok = reasonGenerated, true
		}
		if !ok {
			kept = append(kept, d)
			continue
		}
		suppressed = append(suppressed, model.Suppressed{ID: d.ID, Package: d.Package, Message: d.Message, Reason: reason})
	}
	return kept, suppressed
}

// UnusedIgnores reports, one line per entry, the ignore rules of cfg whose
// rule and package glob match none of suppressed. A stale entry usually means
// the finding it accepted no longer exists; it is a warning, never an error.
// An entry shadowed by an earlier one covering the same finding counts as
// used, since the finding it names is real.
func UnusedIgnores(cfg config.Config, suppressed []model.Suppressed) []string {
	var out []string
	for i, r := range cfg.Ignore {
		used := false
		for _, s := range suppressed {
			if s.ID == r.Rule && config.MatchGlob(r.Package, s.Package) {
				used = true
				break
			}
		}
		if !used {
			out = append(out, fmt.Sprintf("ignore[%d]: %s on %s silenced no finding (%s)", i, r.Rule, r.Package, r.Reason))
		}
	}
	return out
}
