package diagnose

import (
	"context"
	"fmt"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

type godPackage struct{}

func (godPackage) Check(_ context.Context, snap *model.Snapshot, cfg config.Config) []model.Diagnostic {
	var out []model.Diagnostic
	for _, p := range snap.Packages {
		if p.Ce < cfg.GodCeThreshold {
			continue
		}
		out = append(out, model.Diagnostic{
			ID:       "god-package",
			Severity: model.SeverityWarning,
			Package:  p.Path,
			Message:  fmt.Sprintf("package imports %d packages (threshold %d) and knows too much", p.Ce, cfg.GodCeThreshold),
			Evidence: map[string]any{"ce": p.Ce, "imports": p.Imports},
		})
	}
	return out
}
