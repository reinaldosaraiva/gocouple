package main

import (
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/spf13/cobra"
)

type analysisFlags struct {
	config          string
	dir             string
	includeExternal bool
	includeTests    bool
	exportedOnly    bool
	volatilitySince string
}

func (f *analysisFlags) register(cmd *cobra.Command, dirDefault string) {
	fl := cmd.Flags()
	fl.StringVar(&f.config, "config", "", "path to a "+config.FileName+" file (default: "+config.FileName+" in --dir)")
	fl.StringVar(&f.dir, "dir", dirDefault, "module root directory")
	fl.BoolVar(&f.includeExternal, "include-external", false, "count third-party dependencies in Ce")
	fl.BoolVar(&f.includeTests, "include-tests", false, "include test packages")
	fl.BoolVar(&f.exportedOnly, "exported-only", false, "count only exported types in Nc and Na")
}

func (f *analysisFlags) registerVolatility(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.volatilitySince, "volatility-since", "", "measure package churn since a duration (180d) or a date (2026-01-01); empty turns it off")
}

// resolve merges defaults, the YAML file and any flag that was set.
func (f *analysisFlags) resolve(cmd *cobra.Command) (config.Config, error) {
	cfg, err := config.Discover(f.dir, f.config)
	if err != nil {
		return config.Config{}, err
	}
	flags := cmd.Flags()
	if flags.Changed("include-external") {
		cfg.IncludeExternal = f.includeExternal
	}
	if flags.Changed("exported-only") {
		cfg.ExportedOnly = f.exportedOnly
	}
	if flags.Changed("volatility-since") {
		cfg.VolatilitySince = strings.TrimSpace(f.volatilitySince)
	}
	return cfg, nil
}
