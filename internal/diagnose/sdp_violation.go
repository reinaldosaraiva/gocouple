package diagnose

import (
	"context"
	"fmt"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

type sdpViolation struct{}

func (sdpViolation) Check(_ context.Context, snap *model.Snapshot, cfg config.Config) []model.Diagnostic {
	byPath := make(map[string]model.Package, len(snap.Packages))
	for _, p := range snap.Packages {
		byPath[p.Path] = p
	}
	var out []model.Diagnostic
	for _, p := range snap.Packages {
		if p.IsMain || cfg.IsCompositionRoot(p.Path) {
			continue
		}
		for _, imp := range p.Imports {
			q, ok := byPath[imp]
			if !ok || float64(q.Instability) <= float64(p.Instability)+cfg.SDPTolerance {
				continue
			}
			out = append(out, model.Diagnostic{
				ID:       "sdp-violation",
				Severity: model.SeverityWarning,
				Package:  p.Path,
				Message: fmt.Sprintf("depends on %s, which is less stable (I %.2f > %.2f)",
					q.Path, float64(q.Instability), float64(p.Instability)),
				Evidence: map[string]any{
					"from": p.Path, "to": q.Path,
					"from_instability": p.Instability, "to_instability": q.Instability,
				},
			})
		}
	}
	return out
}
