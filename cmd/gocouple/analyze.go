package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/analysis"
	"github.com/reinaldosaraiva/gocouple/internal/model"
	"github.com/reinaldosaraiva/gocouple/internal/output"
	"github.com/spf13/cobra"
)

var formats = map[string]func(io.Writer, *model.Snapshot) error{
	"table":    output.Table,
	"json":     output.JSON,
	"csv":      output.CSV,
	"markdown": output.Markdown,
	"dot":      output.Dot,
	"mermaid":  output.Mermaid,
}

func formatNames() []string {
	return []string{"table", "json", "csv", "markdown", "dot", "mermaid"}
}

type analyzeFlags struct {
	analysisFlags
	format string
	out    string
}

func newAnalyzeCmd() *cobra.Command {
	var f analyzeFlags
	cmd := &cobra.Command{
		Use:   "analyze [patterns...]",
		Short: "Measure package coupling of a Go module",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(cmd, f, args)
		},
	}
	f.register(cmd, ".")
	cmd.Flags().StringVar(&f.format, "format", "table", "output format: "+strings.Join(formatNames(), "|"))
	cmd.Flags().StringVar(&f.out, "out", "", "write output to this file instead of stdout")
	return cmd
}

func runAnalyze(cmd *cobra.Command, f analyzeFlags, patterns []string) error {
	write, ok := formats[f.format]
	if !ok {
		return fmt.Errorf("unsupported format %q (want %s)", f.format, strings.Join(formatNames(), ", "))
	}
	cfg, err := f.resolve(cmd)
	if err != nil {
		return err
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
	return writeTo(cmd.OutOrStdout(), f.out, func(w io.Writer) error { return write(w, snap) })
}

// writeTo streams to stdout, or to path through a temporary file that is
// renamed on success so a failed run never leaves a truncated output.
func writeTo(stdout io.Writer, path string, write func(io.Writer) error) error {
	if path == "" {
		return write(stdout)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	if err := write(tmp); err != nil {
		return errors.Join(err, tmp.Close(), os.Remove(tmp.Name()))
	}
	if err := tmp.Sync(); err != nil {
		return errors.Join(fmt.Errorf("syncing %s: %w", path, err), tmp.Close(), os.Remove(tmp.Name()))
	}
	if err := tmp.Close(); err != nil {
		return errors.Join(fmt.Errorf("writing %s: %w", path, err), os.Remove(tmp.Name()))
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return errors.Join(fmt.Errorf("writing %s: %w", path, err), os.Remove(tmp.Name()))
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return errors.Join(fmt.Errorf("writing %s: %w", path, err), os.Remove(tmp.Name()))
	}
	return nil
}
