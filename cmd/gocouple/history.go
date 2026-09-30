package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/reinaldosaraiva/gocouple/internal/gitrun"

	"github.com/reinaldosaraiva/gocouple/internal/analysis"
	"github.com/reinaldosaraiva/gocouple/internal/history"
	"github.com/reinaldosaraiva/gocouple/internal/model"
	"github.com/reinaldosaraiva/gocouple/internal/output"
	"github.com/spf13/cobra"
)

type historyFlags struct {
	analysisFlags
	repo        string
	branch      string
	since       string
	until       string
	every       int
	max         int
	firstParent bool
	out         string
	csv         string
	workers     int
	noCache     bool
}

func newHistoryCmd() *cobra.Command {
	var f historyFlags
	cmd := &cobra.Command{
		Use:   "history [patterns...]",
		Short: "Track coupling metrics over the git history",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistory(cmd, f, args)
		},
	}
	f.register(cmd, "")
	fl := cmd.Flags()
	fl.StringVar(&f.repo, "repo", ".", "git repository")
	fl.StringVar(&f.branch, "branch", "HEAD", "branch or revision to walk")
	fl.StringVar(&f.since, "since", "", "only commits after this date")
	fl.StringVar(&f.until, "until", "", "only commits before this date")
	fl.IntVar(&f.every, "every", 1, "analyze one of every N commits")
	fl.IntVar(&f.max, "max", 50, "analyze at most N commits (the newest)")
	fl.BoolVar(&f.firstParent, "first-parent", true, "follow only first parents")
	fl.StringVar(&f.out, "out", "history.json", "history JSON output file")
	fl.StringVar(&f.csv, "csv", "", "also write a CSV with one row per commit and package")
	fl.IntVar(&f.workers, "workers", 0, "parallel workers (default: half of the CPUs, minimum 1)")
	fl.BoolVar(&f.noCache, "no-cache", false, "ignore and do not write the cache")
	return cmd
}

func runHistory(cmd *cobra.Command, f historyFlags, patterns []string) error {
	repo, err := filepath.Abs(f.repo)
	if err != nil {
		return fmt.Errorf("resolving --repo: %w", err)
	}
	if f.dir == "" {
		f.dir = repo
	}
	cfg, err := f.resolve(cmd)
	if err != nil {
		return err
	}

	cfg.VolatilitySince = ""

	analyze := func(ctx context.Context, moduleDir string) (*model.Snapshot, error) {
		return analysis.Run(ctx, analysis.Options{
			Dir:          moduleDir,
			Patterns:     patterns,
			IncludeTests: f.includeTests,
			Config:       cfg,
			ToolVersion:  version,
		})
	}
	h, stats, err := history.Run(cmd.Context(), gitrun.Exec{}, analyze, history.Options{
		Repo: repo,
		Dir:  f.dir,
		Selection: history.Selection{
			Branch: f.branch, Since: f.since, Until: f.until,
			Every: f.every, Max: f.max, FirstParent: f.firstParent,
		},
		Workers:  f.workers,
		CacheDir: filepath.Join(repo, ".gocouple", "cache"),
		CacheKey: history.ConfigHash(version, cfg, f.includeTests, patterns),
		NoCache:  f.noCache,
		Progress: cmd.ErrOrStderr(),
	})
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "history: %d analyzed, %d cached, %d failed\n", stats.Analyzed, stats.Cached, stats.Failed); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}
	if err := writeTo(cmd.OutOrStdout(), f.out, func(w io.Writer) error { return output.HistoryJSON(w, h) }); err != nil {
		return err
	}
	if f.csv == "" {
		return nil
	}
	return writeTo(cmd.OutOrStdout(), f.csv, func(w io.Writer) error { return output.HistoryCSV(w, h) })
}
