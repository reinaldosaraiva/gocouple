package diagnose

import (
	"context"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

type uselessnessZone struct{}

func (uselessnessZone) Check(_ context.Context, snap *model.Snapshot, _ config.Config) []model.Diagnostic {
	var out []model.Diagnostic
	for _, p := range snap.Packages {
		if p.Zone != model.ZoneUselessness {
			continue
		}
		out = append(out, model.Diagnostic{
			ID:       "uselessness-zone",
			Severity: model.SeverityWarning,
			Package:  p.Path,
			Message:  "abstract and unstable package; consider removing abstractions that have no consumers",
			Evidence: map[string]any{
				"ca": p.Ca, "ce": p.Ce, "instability": p.Instability,
				"abstractness": p.Abstractness, "distance": p.Distance,
			},
		})
	}
	return out
}
