package diagnose

import (
	"context"
	"fmt"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

type painZone struct{}

func (painZone) Check(_ context.Context, snap *model.Snapshot, cfg config.Config) []model.Diagnostic {
	var out []model.Diagnostic
	for _, p := range snap.Packages {
		if p.Zone != model.ZonePain || p.Ca < cfg.MinCaForPain {
			continue
		}
		out = append(out, model.Diagnostic{
			ID:       "pain-zone",
			Severity: model.SeverityWarning,
			Package:  p.Path,
			Message: fmt.Sprintf("stable and concrete package used by %d packages; introduce a contract (interface) "+
				"consumed by the dependents or inject the dependency", p.Ca),
			Evidence: map[string]any{
				"ca": p.Ca, "ce": p.Ce, "instability": p.Instability,
				"abstractness": p.Abstractness, "distance": p.Distance, "imported_by": p.ImportedBy,
			},
		})
	}
	return out
}
