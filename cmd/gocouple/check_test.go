package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reinaldosaraiva/gocouple/internal/output"
)

func exitOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ec *exitCodeError
	if !errors.As(err, &ec) {
		t.Fatalf("error without exit code: %v", err)
	}
	return ec.code
}

func TestCheckExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"refactored passes", []string{"--dir", fixtureDir("orders-refactored")}, 0},
		{"legacy violates", []string{"--dir", fixtureDir("orders-legacy")}, 1},
		{"legacy tolerated by raising both limits", []string{"--dir", fixtureDir("orders-legacy"), "--max-pain-packages", "1", "--max-distance", "1"}, 0},
		{"pain limit alone is not enough", []string{"--dir", fixtureDir("orders-legacy"), "--max-pain-packages", "1"}, 1},
		{"refactored has no diagnostics at any level", []string{"--dir", fixtureDir("orders-refactored"), "--fail-on", "info"}, 0},
		{"unknown flag", []string{"--nope"}, 2},
		{"bad severity", []string{"--dir", fixtureDir("orders-refactored"), "--fail-on", "fatal"}, 2},
		{"missing baseline", []string{"--dir", fixtureDir("orders-refactored"), "--baseline", "/nonexistent.json"}, 2},
		{"no module", []string{"--dir", t.TempDir()}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := runCLI(t, append([]string{"check"}, tt.args...)...)
			if got := exitOf(t, err); got != tt.want {
				t.Errorf("exit = %d, want %d (err %v)", got, tt.want, err)
			}
		})
	}
}

func TestCheckLegacyReportsViolations(t *testing.T) {
	out, err := runCLI(t, "check", "--dir", fixtureDir("orders-legacy"))
	if exitOf(t, err) != 1 {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{"FAIL max-pain-packages", "FAIL max-distance", "internal/logger", "internal/contracts", "3 violation(s)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestCheckBaselineDetectsRegression(t *testing.T) {
	dir := t.TempDir()
	refBase := filepath.Join(dir, "ref.json")
	legBase := filepath.Join(dir, "leg.json")
	for fx, path := range map[string]string{"orders-refactored": refBase, "orders-legacy": legBase} {
		if _, err := runCLI(t, "analyze", "--dir", fixtureDir(fx), "--format", "json", "--out", path); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runCLI(t, "check", "--dir", fixtureDir("orders-legacy"), "--baseline", refBase)
	if exitOf(t, err) != 1 || !strings.Contains(out, "new package in the pain zone") {
		t.Errorf("legacy vs refactored baseline: err=%v\n%s", err, out)
	}
	if out, err := runCLI(t, "check", "--dir", fixtureDir("orders-refactored"), "--baseline", legBase); err != nil {
		t.Errorf("improvement must pass: %v\n%s", err, out)
	}
	if out, err := runCLI(t, "check", "--dir", fixtureDir("orders-legacy"), "--baseline", legBase); err != nil {
		t.Errorf("unchanged debt must pass: %v\n%s", err, out)
	}
}

func TestCheckGitHubAnnotations(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	out, err := runCLI(t, "check", "--dir", fixtureDir("orders-legacy"))
	if exitOf(t, err) != 1 {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(out, "::error title=gocouple max-distance::") {
		t.Errorf("annotation missing:\n%s", out)
	}
}

func TestAnnotationEscaping(t *testing.T) {
	var b strings.Builder
	err := output.WriteAnnotations(&b, []output.Annotation{{Level: "error", Title: "a,b:c", Message: "100%\nnext\r"}, {Level: "weird", Title: "t", Message: "m"}})
	if err != nil {
		t.Fatal(err)
	}
	want := "::error title=a%2Cb%3Ac::100%25%0Anext%0D\n::warning title=t::m\n"
	if b.String() != want {
		t.Errorf("annotations = %q, want %q", b.String(), want)
	}
}

func TestDogfooding(t *testing.T) {
	if testing.Short() {
		t.Skip("dogfooding loads the whole repository")
	}
	out, err := runCLI(t, "check", "--dir", filepath.Join("..", ".."), "./...")
	if err != nil {
		t.Fatalf("gocouple check ./... must pass on this repository: %v\n%s", err, out)
	}
	if !strings.Contains(out, "gocouple check: OK") {
		t.Errorf("output:\n%s", out)
	}
}
