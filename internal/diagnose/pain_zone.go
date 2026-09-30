package diagnose

import (
	"context"
	"fmt"
	"time"

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
		msg := fmt.Sprintf("stable and concrete package used by %d packages; introduce a contract (interface) "+
			"consumed by the dependents or inject the dependency", p.Ca)
		evidence := map[string]any{
			"ca": p.Ca, "ce": p.Ce, "instability": p.Instability,
			"abstractness": p.Abstractness, "distance": p.Distance, "imported_by": p.ImportedBy,
		}
		if since := volatilityDate(snap); since != "" && p.Churn > 0 {
			msg += fmt.Sprintf("; changed %d times since %s", p.Churn, since)
			evidence["churn"] = p.Churn
		}
		out = append(out, model.Diagnostic{
			ID:       "pain-zone",
			Severity: model.SeverityWarning,
			Package:  p.Path,
			Message:  msg,
			Evidence: evidence,
		})
	}
	return out
}

func volatilityDate(snap *model.Snapshot) string {
	if snap.Config.VolatilitySince == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, snap.Config.VolatilitySince)
	if err != nil {
		return ""
	}
	return t.UTC().Format(time.DateOnly)
}
