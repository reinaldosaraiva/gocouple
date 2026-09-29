package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// Analyzer measures the module found at moduleDir.
type Analyzer func(ctx context.Context, moduleDir string) (*model.Snapshot, error)

// Options configures a history run.
type Options struct {
	Repo      string
	Dir       string
	Selection Selection
	Workers   int
	CacheDir  string
	CacheKey  string
	NoCache   bool
	Progress  io.Writer
}

// Stats reports how the entries were produced.
type Stats struct {
	Analyzed int
	Cached   int
	Failed   int
}

// Run analyzes the selected commits, each in a detached worktree that is
// removed on success, failure and cancellation. The working tree of the
// user's repository is never modified.
func Run(ctx context.Context, git GitRunner, analyze Analyzer, opts Options) (*model.History, Stats, error) {
	top, err := topLevel(ctx, git, opts.Repo)
	if err != nil {
		return nil, Stats{}, err
	}
	rel, err := moduleRel(top, opts.Dir, opts.Repo)
	if err != nil {
		return nil, Stats{}, err
	}
	commits, err := ListCommits(ctx, git, opts.Repo, opts.Selection)
	if err != nil {
		return nil, Stats{}, err
	}

	p := &pool{
		git: git, analyze: analyze, opts: opts, top: top, rel: rel, total: len(commits),
		cache: cache{dir: opts.CacheDir, hash: cacheKey(opts.CacheKey, rel)},
	}
	entries := make([]model.Entry, len(commits))
	jobs := make(chan int)
	workers := opts.Workers
	if workers < 1 {
		workers = max(runtime.NumCPU()/2, 1)
	}
	var wg sync.WaitGroup
	for range min(workers, max(len(commits), 1)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				entries[i] = p.process(ctx, commits[i])
			}
		}()
	}
	for i := range commits {
		select {
		case jobs <- i:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	wg.Wait()

	cleanup := context.WithoutCancel(ctx)
	prunErr := p.prune(cleanup)
	if err := ctx.Err(); err != nil {
		return nil, Stats{}, errors.Join(fmt.Errorf("history interrupted: %w", err), prunErr)
	}
	if prunErr != nil {
		return nil, Stats{}, prunErr
	}

	h := &model.History{SchemaVersion: model.SchemaVersion, Snapshots: entries}
	for _, e := range entries {
		if e.Snapshot != nil {
			h.Module = e.Snapshot.Module
			break
		}
	}
	return h, Stats{Analyzed: int(p.analyzed.Load()), Cached: int(p.cached.Load()), Failed: int(p.failed.Load())}, nil
}

type pool struct {
	git     GitRunner
	analyze Analyzer
	opts    Options
	top     string
	rel     string
	total   int
	cache   cache

	wtMu     sync.Mutex
	progMu   sync.Mutex
	done     atomic.Int64
	analyzed atomic.Int64
	cached   atomic.Int64
	failed   atomic.Int64
}

func (p *pool) process(ctx context.Context, c model.Commit) model.Entry {
	entry := model.Entry{Commit: c}
	note := ""
	switch snap, hit, err := p.snapshot(ctx, c); {
	case err != nil:
		entry.Error = err.Error()
		p.failed.Add(1)
		note = "error: " + firstLine(err.Error())
	case hit:
		entry.Snapshot = snap
		p.cached.Add(1)
		note = "cached"
	default:
		entry.Snapshot = snap
		p.analyzed.Add(1)
	}
	p.report(c, note)
	return entry
}

func (p *pool) snapshot(ctx context.Context, c model.Commit) (*model.Snapshot, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if !p.opts.NoCache {
		if snap, ok := p.cache.load(c.SHA); ok {
			snap.Commit = &c
			return snap, true, nil
		}
	}
	snap, err := p.inWorktree(ctx, c.SHA)
	if err != nil {
		return nil, false, err
	}
	snap.Commit = &c
	if !p.opts.NoCache {
		if err := p.cache.store(c.SHA, snap); err != nil {
			p.warn(c, "cache write failed: "+firstLine(err.Error()))
		}
	}
	return snap, false, nil
}

func (p *pool) inWorktree(ctx context.Context, sha string) (snap *model.Snapshot, err error) {
	dir, err := os.MkdirTemp("", "gocouple-wt-*")
	if err != nil {
		return nil, fmt.Errorf("creating worktree dir: %w", err)
	}
	p.wtMu.Lock()
	_, addErr := p.git.Run(ctx, p.top, "worktree", "add", "--detach", dir, sha)
	p.wtMu.Unlock()
	defer func() {
		var rmErr error
		if addErr == nil {
			cleanup := context.WithoutCancel(ctx)
			p.wtMu.Lock()
			_, rmErr = p.git.Run(cleanup, p.top, "worktree", "remove", "--force", dir)
			p.wtMu.Unlock()
		}
		err = errors.Join(err, rmErr, os.RemoveAll(dir))
	}()
	if addErr != nil {
		return nil, fmt.Errorf("creating worktree for %s: %w", shortSHA(sha), addErr)
	}
	snap, err = p.analyze(ctx, filepath.Join(dir, p.rel))
	if err != nil {
		return nil, err
	}
	return snap, nil
}

func (p *pool) prune(ctx context.Context) error {
	p.wtMu.Lock()
	defer p.wtMu.Unlock()
	if _, err := p.git.Run(ctx, p.top, "worktree", "prune"); err != nil {
		return fmt.Errorf("pruning worktrees: %w", err)
	}
	return nil
}

func (p *pool) report(c model.Commit, note string) {
	if p.opts.Progress == nil {
		return
	}
	if note != "" {
		note = " (" + note + ")"
	}
	p.progMu.Lock()
	defer p.progMu.Unlock()
	n := p.done.Add(1)
	_, _ = fmt.Fprintf(p.opts.Progress, "[%d/%d] %s %s %s%s\n", n, p.total, shortSHA(c.SHA), c.Date, c.Subject, note)
}

func (p *pool) warn(c model.Commit, msg string) {
	if p.opts.Progress == nil {
		return
	}
	p.progMu.Lock()
	defer p.progMu.Unlock()
	_, _ = fmt.Fprintf(p.opts.Progress, "warning: %s: %s\n", shortSHA(c.SHA), msg)
}

func topLevel(ctx context.Context, git GitRunner, repo string) (string, error) {
	out, err := git.Run(ctx, repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("locating repository at %s: %w", repo, err)
	}
	return filepath.FromSlash(strings.TrimSpace(string(out))), nil
}

func moduleRel(top, dir, repo string) (string, error) {
	if dir == "" {
		dir = repo
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", dir, err)
	}
	real := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return p
	}
	rel, err := filepath.Rel(real(top), real(abs))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("module directory %s is outside repository %s", abs, top)
	}
	return rel, nil
}

func cacheKey(key, rel string) string {
	sum := sha256.Sum256([]byte(key + "|" + filepath.ToSlash(rel)))
	return hex.EncodeToString(sum[:])[:12]
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
