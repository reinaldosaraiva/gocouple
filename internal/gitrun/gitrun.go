package gitrun

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Runner runs one git command in dir and returns its standard output.
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

// Exec runs the git binary found in PATH.
type Exec struct{}

// Run implements Runner.
func (Exec) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
