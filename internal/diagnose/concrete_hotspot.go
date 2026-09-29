package diagnose

import (
	"context"
	"fmt"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

type concreteHotspot struct{}

func (concreteHotspot) Check(_ context.Context, snap *model.Snapshot, cfg config.Config) []model.Diagnostic {
	var out []model.Diagnostic
	for _, p := range snap.Packages {
		if p.Ca < cfg.HotspotCaThreshold || p.Abstractness != 0 {
			continue
		}
		out = append(out, model.Diagnostic{
			ID:       "concrete-hotspot",
			Severity: model.SeverityWarning,
			Package:  p.Path,
			Message:  fmt.Sprintf("package without abstractions is imported directly by %d packages", p.Ca),
			Evidence: map[string]any{"ca": p.Ca, "abstractness": p.Abstractness, "imported_by": p.ImportedBy},
		})
	}
	return out
}
