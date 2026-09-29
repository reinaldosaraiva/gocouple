package analysis

import (
	"context"
	"fmt"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/diagnose"
	"github.com/reinaldosaraiva/gocouple/internal/loader"
	"github.com/reinaldosaraiva/gocouple/internal/metrics"
	"github.com/reinaldosaraiva/gocouple/internal/model"
	"github.com/reinaldosaraiva/gocouple/internal/typesusage"
)

// Options selects the module and the effective configuration to analyze.
type Options struct {
	Dir          string
	Patterns     []string
	IncludeTests bool
	Config       config.Config
	ToolVersion  string
}

// Run loads the module in opts.Dir, measures it and runs every rule.
func Run(ctx context.Context, opts Options) (*model.Snapshot, error) {
	cfg := opts.Config
	loaded, err := loader.Load(ctx, loader.Options{
		Dir:             opts.Dir,
		Patterns:        opts.Patterns,
		IncludeExternal: cfg.IncludeExternal,
		IncludeTests:    opts.IncludeTests,
		ExportedOnly:    cfg.ExportedOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("analyzing %s: %w", opts.Dir, err)
	}
	loaded = loader.Exclude(loaded, cfg.IsExcluded)
	res := metrics.Analyze(loaded.Packages, cfg.DistanceThreshold)
	snap := &model.Snapshot{
		SchemaVersion: model.SchemaVersion,
		ToolVersion:   opts.ToolVersion,
		Module:        loaded.Module,
		Config: model.Config{
			DistanceThreshold: cfg.DistanceThreshold,
			IncludeExternal:   cfg.IncludeExternal,
			IncludeTests:      opts.IncludeTests,
			ExportedOnly:      cfg.ExportedOnly,
		},
		Packages: res.Packages,
		Cycles:   res.Cycles,
		Summary:  res.Summary,
	}
	snap.Interfaces = typesusage.Analyze(loaded.Typed, cfg.IsCompositionRoot)
	snap.Diagnostics = diagnose.Run(ctx, snap, cfg, diagnose.Rules())
	return snap, nil
}
