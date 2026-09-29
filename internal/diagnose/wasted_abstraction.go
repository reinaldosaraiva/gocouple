package diagnose

import (
	"context"
	"fmt"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

type wastedAbstraction struct{}

func (wastedAbstraction) Check(_ context.Context, snap *model.Snapshot, _ config.Config) []model.Diagnostic {
	var out []model.Diagnostic
	for _, u := range snap.Interfaces {
		var msg string
		switch {
		case len(u.ConcreteRefs) > len(u.IfaceRefs):
			msg = fmt.Sprintf("interface %s exists, but consumers depend on its implementation (%d) rather than on the interface (%d): the dependency was not inverted",
				u.Name, len(u.ConcreteRefs), len(u.IfaceRefs))
		case len(u.IfaceRefs) == 0:
			msg = fmt.Sprintf("interface %s has no consumers outside its package", u.Name)
		default:
			continue
		}
		out = append(out, model.Diagnostic{
			ID:       "wasted-abstraction",
			Severity: model.SeverityWarning,
			Package:  u.Package,
			Message:  msg,
			Evidence: map[string]any{
				"interface":        u.Name,
				"iface_refs":       len(u.IfaceRefs),
				"concrete_refs":    len(u.ConcreteRefs),
				"implementers":     u.Implementers,
				"referenced_by":    u.IfaceRefs,
				"concrete_used_by": u.ConcreteRefs,
			},
		})
	}
	return out
}
