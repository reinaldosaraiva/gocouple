package analysis

import (
	"context"
	"fmt"
	"time"

	"github.com/reinaldosaraiva/gocouple/internal/gitrun"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/diagnose"
	"github.com/reinaldosaraiva/gocouple/internal/loader"
	"github.com/reinaldosaraiva/gocouple/internal/metrics"
	"github.com/reinaldosaraiva/gocouple/internal/model"
	"github.com/reinaldosaraiva/gocouple/internal/typesusage"
	"github.com/reinaldosaraiva/gocouple/internal/volatility"
)

// Options selects the module and the effective configuration to analyze.
type Options struct {
	Dir          string
	Patterns     []string
	IncludeTests bool
	Config       config.Config
	ToolVersion  string
	Now          time.Time
	Git          gitrun.Runner
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
	warnings, err := applyVolatility(ctx, snap, loaded, cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("analyzing %s: %w", opts.Dir, err)
	}
	snap.Interfaces = typesusage.Analyze(loaded.Typed, cfg.IsCompositionRoot)
	diags := diagnose.Run(ctx, snap, cfg, diagnose.Rules())
	var suppressed []model.Suppressed
	snap.Diagnostics, suppressed = diagnose.Suppress(snap, cfg, diags)
	if len(suppressed) > 0 {
		snap.Suppressed = suppressed
	}
	snap.Warnings = append(diagnose.UnusedIgnores(cfg, suppressed), warnings...)
	return snap, nil
}

func applyVolatility(ctx context.Context, snap *model.Snapshot, loaded loader.Result, cfg config.Config, opts Options) ([]string, error) {
	if cfg.VolatilitySince == "" {
		return nil, nil
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	since, err := volatility.ParseSince(cfg.VolatilitySince, now)
	if err != nil {
		return nil, err
	}
	if loaded.ModuleDir == "" {
		return nil, fmt.Errorf("volatility needs the module directory and the loader reported none")
	}
	git := opts.Git
	if git == nil {
		git = gitrun.Exec{}
	}
	paths := make([]string, len(snap.Packages))
	for i, p := range snap.Packages {
		paths[i] = p.Path
	}
	res, err := volatility.Compute(ctx, git, loaded.ModuleDir, loaded.Module, paths, since)
	if err != nil {
		return nil, err
	}
	if res.Churn == nil {
		return res.Warnings, nil
	}
	peak := 0
	for _, n := range res.Churn {
		peak = max(peak, n)
	}
	for i := range snap.Packages {
		n := res.Churn[snap.Packages[i].Path]
		snap.Packages[i].Churn = n
		if peak > 0 {
			snap.Packages[i].Volatility = model.Ratio(float64(n) / float64(peak))
		}
	}
	snap.Config.VolatilitySince = since.UTC().Format(time.RFC3339)
	return res.Warnings, nil
}
