package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/reinaldosaraiva/gocouple/internal/analysis"
	"github.com/reinaldosaraiva/gocouple/internal/check"
	"github.com/reinaldosaraiva/gocouple/internal/model"
	"github.com/reinaldosaraiva/gocouple/internal/output"
	"github.com/spf13/cobra"
)

const (
	exitViolation = 1
	exitError     = 2
)

// exitCodeError carries the process exit code; a nil err means the reason was
// already printed.
type exitCodeError struct {
	code int
	err  error
}

func (e *exitCodeError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *exitCodeError) Unwrap() error { return e.err }

type checkFlags struct {
	analysisFlags
	maxDistance  float64
	maxPain      int
	failOnCycles bool
	failOn       string
	baseline     string
	tolerance    float64
}

func newCheckCmd() *cobra.Command {
	var f checkFlags
	cmd := &cobra.Command{
		Use:   "check [patterns...]",
		Short: "Fail when coupling thresholds are violated (exit 0 ok, 1 violation, 2 error)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runCheck(cmd, f, args); err != nil {
				var ec *exitCodeError
				if errors.As(err, &ec) {
					return err
				}
				return &exitCodeError{code: exitError, err: err}
			}
			return nil
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &exitCodeError{code: exitError, err: err}
	})
	f.register(cmd, ".")
	f.registerVolatility(cmd)
	fl := cmd.Flags()
	fl.Float64Var(&f.maxDistance, "max-distance", 0, "maximum distance of a relevant package (default from config, 0.7)")
	fl.IntVar(&f.maxPain, "max-pain-packages", 0, "maximum allowed pain-zone packages (default from config, 0)")
	fl.BoolVar(&f.failOnCycles, "fail-on-cycles", true, "fail when a dependency cycle exists")
	fl.StringVar(&f.failOn, "fail-on", "error", "lowest diagnostic severity that fails: info|warning|error")
	fl.StringVar(&f.baseline, "baseline", "", "analysis.json to compare against; only regressions fail")
	fl.Float64Var(&f.tolerance, "tolerance", 0.02, "allowed increase of the average distance over the baseline")
	return cmd
}

func runCheck(cmd *cobra.Command, f checkFlags, patterns []string) error {
	if !check.ValidSeverity(f.failOn) {
		return fmt.Errorf("invalid --fail-on %q (want info, warning or error)", f.failOn)
	}
	cfg, err := f.resolve(cmd)
	if err != nil {
		return err
	}
	opts := check.FromConfig(cfg)
	flags := cmd.Flags()
	if flags.Changed("max-distance") {
		opts.MaxDistance = f.maxDistance
	}
	if flags.Changed("max-pain-packages") {
		opts.MaxPainPackages = f.maxPain
	}
	if flags.Changed("fail-on-cycles") {
		opts.FailOnCycles = f.failOnCycles
	}
	opts.FailOn = f.failOn
	opts.Tolerance = f.tolerance

	var baseline *model.Snapshot
	if f.baseline != "" {
		var b model.Snapshot
		if err := readJSON(f.baseline, &b); err != nil {
			return err
		}
		if b.SchemaVersion != model.SchemaVersion {
			return fmt.Errorf("%s: unsupported schema_version %q (want %q)", f.baseline, b.SchemaVersion, model.SchemaVersion)
		}
		baseline = &b
	}

	snap, err := analysis.Run(cmd.Context(), analysis.Options{
		Dir:          f.dir,
		Patterns:     patterns,
		IncludeTests: f.includeTests,
		Config:       cfg,
		ToolVersion:  version,
	})
	if err != nil {
		return err
	}

	for _, m := range snap.Warnings {
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "gocouple check: warning:", m); err != nil {
			return err
		}
	}
	violations := check.Evaluate(snap, baseline, opts)
	out := cmd.OutOrStdout()
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		var notes []output.Annotation
		for _, v := range violations {
			level := "error"
			if v.Severity == model.SeverityWarning || v.Severity == model.SeverityInfo {
				level = "warning"
			}
			notes = append(notes, output.Annotation{Level: level, Title: "gocouple " + v.Rule, Message: v.Message})
		}
		if err := output.WriteAnnotations(out, notes); err != nil {
			return err
		}
	}
	if len(violations) == 0 {
		_, err := fmt.Fprintf(out, "gocouple check: OK (%d packages, average distance %.2f)\n", snap.Summary.Packages, float64(snap.Summary.AvgDistance))
		return err
	}
	for _, v := range violations {
		if _, err := fmt.Fprintf(out, "FAIL %s: %s\n", v.Rule, v.Message); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(out, "gocouple check: %d violation(s)\n", len(violations)); err != nil {
		return err
	}
	return &exitCodeError{code: exitViolation}
}
