package volatility

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/reinaldosaraiva/gocouple/internal/gitrun"
)

// Result is the churn per package and the environment problems that kept
// the measurement from being complete.
type Result struct {
	Churn    map[string]int
	Warnings []string
}

var durationDays = regexp.MustCompile(`^(\d+)d$`)

// ParseSince resolves a duration in days (180d), a date (2026-01-01) or an
// RFC3339 time to an absolute time, using now for durations.
func ParseSince(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("volatility window is empty")
	}
	if m := durationDays.FindStringSubmatch(value); m != nil {
		days, err := strconv.Atoi(m[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("volatility window %q: %w", value, err)
		}
		return now.UTC().AddDate(0, 0, -days), nil
	}
	if t, err := time.Parse(time.DateOnly, value); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("volatility window %q: want a duration like 180d, a date like 2026-01-01 or an RFC3339 time", value)
}

// Compute counts, for every path of pkgPaths, the non-merge commits since
// that touched at least one non-test .go file directly in the package
// directory. Every path is present in the result, zero when untouched. A
// missing repository or a shallow clone yields warnings and no churn map;
// only a failing git log is an error.
func Compute(ctx context.Context, git gitrun.Runner, moduleDir, module string, pkgPaths []string, since time.Time) (Result, error) {
	inside, err := git.Run(ctx, moduleDir, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(string(inside)) != "true" {
		return Result{Warnings: []string{"volatility skipped: " + moduleDir + " is not inside a git work tree"}}, nil
	}
	shallow, err := git.Run(ctx, moduleDir, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return Result{Warnings: []string{"volatility skipped: cannot detect a shallow repository: " + err.Error()}}, nil
	}
	if strings.TrimSpace(string(shallow)) == "true" {
		return Result{Warnings: []string{"volatility skipped: shallow repository, history is incomplete"}}, nil
	}

	// git log --since-as-filter keeps commits dated inside the window even
	// behind an older-dated one; plain --since stops at the first older commit.
	out, err := git.Run(ctx, moduleDir, "log", "--since-as-filter="+since.UTC().Format(time.RFC3339), "--no-merges", "--name-only", "--relative", "--format=%x1e%H", "--", ".")
	if err != nil {
		return Result{}, fmt.Errorf("reading history of %s: %w", moduleDir, err)
	}

	known := make(map[string]bool, len(pkgPaths))
	churn := make(map[string]int, len(pkgPaths))
	for _, p := range pkgPaths {
		known[p] = true
		churn[p] = 0
	}
	for record := range strings.SplitSeq(string(out), "\x1e") {
		touched := map[string]bool{}
		for line := range strings.SplitSeq(record, "\n") {
			line = unquotePath(strings.TrimRight(line, "\r"))
			if !strings.HasSuffix(line, ".go") || strings.HasSuffix(line, "_test.go") {
				continue
			}
			pkg := module
			if dir := path.Dir(line); dir != "." {
				pkg = module + "/" + dir
			}
			if known[pkg] {
				touched[pkg] = true
			}
		}
		for pkg := range touched {
			churn[pkg]++
		}
	}
	return Result{Churn: churn}, nil
}

func unquotePath(line string) string {
	if len(line) < 2 || line[0] != '"' || line[len(line)-1] != '"' {
		return line
	}
	if unquoted, err := strconv.Unquote(line); err == nil {
		return unquoted
	}
	return line
}
