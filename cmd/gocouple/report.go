package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/reinaldosaraiva/gocouple/internal/model"
	"github.com/reinaldosaraiva/gocouple/internal/report"
	"github.com/spf13/cobra"
)

type reportFlags struct {
	history  string
	snapshot string
	out      string
}

func newReportCmd() *cobra.Command {
	var f reportFlags
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render a self-contained HTML report from history and/or snapshot JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReport(cmd, f)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.history, "history", "", "history.json produced by `gocouple history`")
	fl.StringVar(&f.snapshot, "snapshot", "", "analysis.json produced by `gocouple analyze --format json`")
	fl.StringVar(&f.out, "out", "report.html", "output HTML file")
	return cmd
}

func runReport(cmd *cobra.Command, f reportFlags) error {
	if f.history == "" && f.snapshot == "" {
		return errors.New("report needs --history and/or --snapshot")
	}
	var in report.Input
	if f.history != "" {
		var h model.History
		if err := readJSON(f.history, &h); err != nil {
			return err
		}
		if h.SchemaVersion != model.SchemaVersion {
			return fmt.Errorf("%s: unsupported schema_version %q (want %q)", f.history, h.SchemaVersion, model.SchemaVersion)
		}
		in.History = &h
	}
	if f.snapshot != "" {
		var s model.Snapshot
		if err := readJSON(f.snapshot, &s); err != nil {
			return err
		}
		if s.SchemaVersion != model.SchemaVersion {
			return fmt.Errorf("%s: unsupported schema_version %q (want %q)", f.snapshot, s.SchemaVersion, model.SchemaVersion)
		}
		in.Snapshot = &s
	}
	return writeTo(cmd.OutOrStdout(), f.out, func(w io.Writer) error { return report.Render(w, in) })
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}
