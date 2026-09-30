package history

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/reinaldosaraiva/gocouple/internal/gitrun"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

const logFormat = "--format=%H%x1f%cI%x1f%s"

// Selection chooses which commits to analyze.
type Selection struct {
	Branch      string
	Since       string
	Until       string
	Every       int
	Max         int
	FirstParent bool
}

// ListCommits returns the selected commits oldest first. Sampling starts at
// the newest commit and keeps one of every Selection.Every, then keeps at
// most Selection.Max of the newest ones.
func ListCommits(ctx context.Context, git gitrun.Runner, repo string, sel Selection) ([]model.Commit, error) {
	branch := sel.Branch
	if branch == "" {
		branch = "HEAD"
	}
	args := []string{"log", logFormat}
	if sel.FirstParent {
		args = append(args, "--first-parent")
	}
	if sel.Since != "" {
		args = append(args, "--since="+sel.Since)
	}
	if sel.Until != "" {
		args = append(args, "--until="+sel.Until)
	}
	args = append(args, branch, "--")
	out, err := git.Run(ctx, repo, args...)
	if err != nil {
		return nil, fmt.Errorf("listing commits of %s: %w", branch, err)
	}
	all, err := parseLog(string(out))
	if err != nil {
		return nil, err
	}

	every := max(sel.Every, 1)
	var picked []model.Commit
	for i := 0; i < len(all); i += every {
		picked = append(picked, all[i])
		if sel.Max > 0 && len(picked) == sel.Max {
			break
		}
	}
	for i, j := 0, len(picked)-1; i < j; i, j = i+1, j-1 {
		picked[i], picked[j] = picked[j], picked[i]
	}
	return picked, nil
}

func parseLog(out string) ([]model.Commit, error) {
	var commits []model.Commit
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\x1f", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("parsing git log line %s", strconv.Quote(line))
		}
		when, err := time.Parse(time.RFC3339, fields[1])
		if err != nil {
			return nil, fmt.Errorf("parsing commit date %q: %w", fields[1], err)
		}
		commits = append(commits, model.Commit{SHA: fields[0], Date: when.UTC().Format(time.RFC3339), Subject: fields[2]})
	}
	return commits, nil
}
