package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func vgit(t *testing.T, dir, date string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func volatilityRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	repo := t.TempDir()
	files := map[string]string{
		"go.mod":       "module example.com/v\n\ngo 1.26\n",
		"hot/hot.go":   "package hot\n\ntype H struct{}\n",
		"cold/cold.go": "package cold\n\ntype C struct{}\n",
	}
	for i := 1; i <= 3; i++ {
		files[fmt.Sprintf("u%d/u.go", i)] = fmt.Sprintf("package u%d\n\nimport (\n\t_ \"example.com/v/cold\"\n\t_ \"example.com/v/hot\"\n)\n", i)
	}
	writeFiles(t, repo, files)
	vgit(t, repo, "2026-01-10T12:00:00Z", "init", "-b", "main")
	vgit(t, repo, "2026-01-10T12:00:00Z", "add", "-A")
	vgit(t, repo, "2026-01-10T12:00:00Z", "commit", "-m", "initial")
	writeFiles(t, repo, map[string]string{"hot/hot.go": "package hot\n\ntype H struct{ N int }\n"})
	vgit(t, repo, "2026-03-05T12:00:00Z", "add", "-A")
	vgit(t, repo, "2026-03-05T12:00:00Z", "commit", "-m", "change hot")
	return repo
}

func TestAnalyzeVolatilityGolden(t *testing.T) {
	repo := volatilityRepo(t)
	for _, tt := range []struct{ format, golden string }{
		{"table", "volatility.table.golden"},
		{"json", "volatility.json.golden"},
		{"csv", "volatility.csv.golden"},
		{"markdown", "volatility.markdown.golden"},
	} {
		out, err := runCLI(t, "analyze", "--dir", repo, "--volatility-since", "2026-03-01", "--format", tt.format)
		if err != nil {
			t.Fatalf("%s: %v", tt.format, err)
		}
		checkGolden(t, tt.golden, out)
	}
}

func TestAnalyzeVolatilityDormantPainIsSuppressed(t *testing.T) {
	repo := volatilityRepo(t)
	out, err := runCLI(t, "analyze", "--dir", repo, "--volatility-since", "2026-03-01")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"pain-zone example.com/v/cold: stable in window (no commit since 2026-03-01)",
		"changed 1 times since 2026-03-01",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[warning] pain-zone example.com/v/cold") {
		t.Errorf("dormant package still reported:\n%s", out)
	}
}

func TestAnalyzeVolatilityOffMatchesPlainAnalysis(t *testing.T) {
	repo := volatilityRepo(t)
	plain, err := runCLI(t, "analyze", "--dir", repo)
	if err != nil {
		t.Fatal(err)
	}
	off, err := runCLI(t, "analyze", "--dir", repo, "--volatility-since", "")
	if err != nil {
		t.Fatal(err)
	}
	if plain != off || strings.Contains(plain, "CHURN") || strings.Contains(plain, "Suppressed") {
		t.Errorf("plain and off differ or leak volatility:\n%s\n---\n%s", plain, off)
	}
}

func TestVolatilityConfigKeyAndFlagPrecedence(t *testing.T) {
	repo := volatilityRepo(t)
	writeFiles(t, repo, map[string]string{".gocouple.yaml": "volatility:\n  since: 2026-03-01\n"})
	out, err := runCLI(t, "analyze", "--dir", repo)
	if err != nil || !strings.Contains(out, "CHURN") {
		t.Fatalf("yaml key did not enable volatility: %v\n%s", err, out)
	}
	out, err = runCLI(t, "analyze", "--dir", repo, "--volatility-since", "")
	if err != nil || strings.Contains(out, "CHURN") {
		t.Errorf("flag must win over yaml: %v\n%s", err, out)
	}
	if _, err := runCLI(t, "analyze", "--dir", repo, "--volatility-since", "soon"); err == nil {
		t.Error("invalid window must fail")
	}
}

func TestVolatilityOutsideGitWarnsAndKeepsOutput(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod": "module example.com/g\n\ngo 1.26\n",
		"a/a.go": "package a\n\ntype T struct{}\n",
	})
	plain, err := runCLI(t, "analyze", "--dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "analyze", "--dir", dir, "--volatility-since", "30d")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "volatility skipped") || strings.Contains(out, "CHURN") {
		t.Errorf("want a warning and no churn column:\n%s", out)
	}
	if !strings.HasPrefix(out, plain) {
		t.Errorf("output before the warning must equal the plain analysis:\n%s\n---\n%s", plain, out)
	}
}

func TestVolatilityAcceptsBaselineFromPriorVersion(t *testing.T) {
	repo := volatilityRepo(t)
	base := filepath.Join(t.TempDir(), "base.json")
	if _, err := runCLI(t, "analyze", "--dir", repo, "--format", "json", "--out", base); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(base)
	if err != nil || strings.Contains(string(data), "churn") || strings.Contains(string(data), "volatility") {
		t.Fatalf("flag-off json must not carry volatility fields (err=%v)", err)
	}
	if out, err := runCLI(t, "check", "--dir", repo, "--volatility-since", "2026-03-01", "--baseline", base); err != nil {
		t.Errorf("baseline without volatility fields must load: %v\n%s", err, out)
	}
}

func TestHistoryIgnoresVolatilityConfigWithNote(t *testing.T) {
	repo := volatilityRepo(t)
	writeFiles(t, repo, map[string]string{".gocouple.yaml": "volatility:\n  since: 2026-03-01\n"})
	var errOut bytes.Buffer
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&errOut)
	root.SetArgs([]string{"history", "--repo", repo, "--out", filepath.Join(t.TempDir(), "h.json"), "--no-cache"})
	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "volatility.since is ignored") {
		t.Errorf("note missing:\n%s", errOut.String())
	}
}

func TestCheckCountsOnlyPainThatChangedInWindow(t *testing.T) {
	repo := volatilityRepo(t)
	out, err := runCLI(t, "check", "--dir", repo)
	if exitOf(t, err) != 1 || !strings.Contains(out, "2 packages in the pain zone") {
		t.Fatalf("without volatility both packages count: err=%v\n%s", err, out)
	}
	out, err = runCLI(t, "check", "--dir", repo, "--volatility-since", "2026-03-01")
	if exitOf(t, err) != 1 || !strings.Contains(out, "1 packages in the pain zone") || strings.Contains(out, "example.com/v/cold") {
		t.Errorf("with volatility only the changed package counts: err=%v\n%s", err, out)
	}
}
