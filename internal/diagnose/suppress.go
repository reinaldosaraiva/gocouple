package diagnose

import (
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
