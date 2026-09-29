package main

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

var update = flag.Bool("update", false, "rewrite golden files")

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(args)
	err := root.ExecuteContext(t.Context())
	return out.String(), err
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "golden", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden (run with -update): %v", err)
	}
	if string(want) != got {
		t.Errorf("output differs from %s (run with -update to accept)\n--- got ---\n%s", name, got)
	}
}

func fixtureDir(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

func TestAnalyzeSimpleGolden(t *testing.T) {
	tests := []struct{ format, golden string }{
		{"table", "simple.table.golden"},
		{"json", "simple.json.golden"},
		{"csv", "simple.csv.golden"},
		{"markdown", "simple.markdown.golden"},
		{"dot", "simple.dot.golden"},
		{"mermaid", "simple.mermaid.golden"},
	}
	analyzeGolden(t, "simple", tests)
}

func TestAnalyzeOrdersGolden(t *testing.T) {
	for _, fx := range []string{"orders-legacy", "orders-refactored"} {
		analyzeGolden(t, fx, []struct{ format, golden string }{
			{"table", fx + ".table.golden"},
			{"json", fx + ".json.golden"},
			{"csv", fx + ".csv.golden"},
			{"markdown", fx + ".markdown.golden"},
			{"dot", fx + ".dot.golden"},
			{"mermaid", fx + ".mermaid.golden"},
		})
	}
}

func analyzeGolden(t *testing.T, fixture string, tests []struct{ format, golden string }) {
	t.Helper()
	for _, tt := range tests {
		t.Run(fixture+"/"+tt.format, func(t *testing.T) {
			t.Parallel()
			got, err := runCLI(t, "analyze", "--dir", fixtureDir(fixture), "--format", tt.format)
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			checkGolden(t, tt.golden, got)
		})
	}
}

func TestAnalyzeOutFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "a.json")
	stdout, err := runCLI(t, "analyze", "--dir", fixtureDir("simple"), "--format", "json", "--out", out)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if fi, err := os.Stat(out); err != nil || fi.Size() == 0 {
		t.Errorf("output file missing or empty: %v", err)
	}
}

func TestAnalyzeRejectsUnknownFormat(t *testing.T) {
	if _, err := runCLI(t, "analyze", "--dir", fixtureDir("simple"), "--format", "xml"); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestOrdersLegacyDiagnosticSet(t *testing.T) {
	out, err := runCLI(t, "analyze", "--dir", fixtureDir("orders-legacy"), "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var snap model.Snapshot
	if err := json.Unmarshal([]byte(out), &snap); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range snap.Diagnostics {
		got = append(got, d.ID+" "+strings.TrimPrefix(d.Package, "example.com/orderslegacy/internal/"))
	}
	want := []string{
		"uselessness-zone contracts",
		"wasted-abstraction contracts",
		"wasted-abstraction contracts",
		"concrete-hotspot logger",
		"pain-zone logger",
		"wasted-abstraction notify",
		"god-package order",
		"wasted-abstraction payment",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
}

func TestOrdersRefactoredIsClean(t *testing.T) {
	out, err := runCLI(t, "analyze", "--dir", fixtureDir("orders-refactored"), "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var snap model.Snapshot
	if err := json.Unmarshal([]byte(out), &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Diagnostics) != 0 || snap.Summary.Pain != 0 || snap.Summary.Cycles != 0 {
		t.Errorf("diagnostics=%v summary=%+v", snap.Diagnostics, snap.Summary)
	}
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestAnalyzeConfigPrecedenceAndExclude(t *testing.T) {
	src := map[string]string{
		"go.mod":     "module example.com/c\n\ngo 1.26\n",
		"a/a.go":     "package a\n\nimport _ \"example.com/c/b\"\nimport _ \"example.com/c/mocks\"\n",
		"b/b.go":     "package b\n\ntype T struct{}\n",
		"mocks/m.go": "package mocks\n",
	}
	dir := writeTree(t, src)

	out, err := runCLI(t, "analyze", "--dir", dir, "--format", "csv")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "example.com/c/mocks") {
		t.Fatalf("mocks package missing without config:\n%s", out)
	}

	src[".gocouple.yaml"] = "exclude: [\"**/mocks/**\"]\ndistance_threshold: 0.9\n"
	dir = writeTree(t, src)
	out, err = runCLI(t, "analyze", "--dir", dir, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var snap model.Snapshot
	if err := json.Unmarshal([]byte(out), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Summary.Packages != 2 || snap.Config.DistanceThreshold != 0.9 {
		t.Errorf("summary=%+v config=%+v", snap.Summary, snap.Config)
	}
	for _, p := range snap.Packages {
		if p.Path == "example.com/c/a" && p.Ce != 1 {
			t.Errorf("a.Ce = %d, excluded import must not count", p.Ce)
		}
	}
}

func TestAnalyzeConfigErrors(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         "module example.com/c\n\ngo 1.26\n",
		"a/a.go":         "package a\n",
		".gocouple.yaml": "distance_treshold: 0.5\n",
	})
	if _, err := runCLI(t, "analyze", "--dir", dir); err == nil || !strings.Contains(err.Error(), "distance_treshold") {
		t.Errorf("err = %v", err)
	}
	if _, err := runCLI(t, "analyze", "--dir", dir, "--config", filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("missing explicit config must fail")
	}
}

func TestAnalyzeFlagOverridesConfig(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         "module example.com/c\n\ngo 1.26\n",
		"a/a.go":         "package a\n\ntype Exported struct{}\n\ntype hidden struct{}\n",
		".gocouple.yaml": "exported_only: true\n",
	})
	nc := func(args ...string) int {
		out, err := runCLI(t, append([]string{"analyze", "--dir", dir, "--format", "json"}, args...)...)
		if err != nil {
			t.Fatal(err)
		}
		var snap model.Snapshot
		if err := json.Unmarshal([]byte(out), &snap); err != nil {
			t.Fatal(err)
		}
		return snap.Packages[0].Nc
	}
	if got := nc(); got != 1 {
		t.Errorf("yaml exported_only: Nc = %d, want 1", got)
	}
	if got := nc("--exported-only=false"); got != 2 {
		t.Errorf("flag must override yaml: Nc = %d, want 2", got)
	}
}

func TestAnalyzeDeterministic(t *testing.T) {
	for _, format := range formatNames() {
		t.Run(format, func(t *testing.T) { t.Parallel(); deterministic(t, format) })
	}
}

func deterministic(t *testing.T, format string) {
	t.Helper()
	first, err := runCLI(t, "analyze", "--dir", fixtureDir("orders-legacy"), "--format", format)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runCLI(t, "analyze", "--dir", fixtureDir("orders-legacy"), "--format", format)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("%s output is not stable", format)
	}
}

func TestOutIsAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("previous"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "analyze", "--dir", fixtureDir("orders-legacy"), "--format", "xml", "--out", target); err == nil {
		t.Fatal("expected error")
	}
	if body, _ := os.ReadFile(target); string(body) != "previous" {
		t.Errorf("failed run overwrote the output: %q", body)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
	if _, err := runCLI(t, "analyze", "--dir", fixtureDir("simple"), "--format", "csv", "--out", target); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(target); !strings.HasPrefix(string(body), "package,ca,") {
		t.Errorf("output = %q", body)
	}
	if _, err := runCLI(t, "analyze", "--dir", fixtureDir("simple"), "--out", filepath.Join(dir, "missing", "x.txt")); err == nil {
		t.Error("expected error for an unwritable path")
	}
}

func TestWriteToKeepsTargetWhenTheWriterFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("previous"), 0o644); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err := writeTo(io.Discard, target, func(w io.Writer) error {
		if _, werr := io.WriteString(w, "partial"); werr != nil {
			return werr
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the writer error", err)
	}
	if body, _ := os.ReadFile(target); string(body) != "previous" {
		t.Errorf("target = %q, want the previous content", body)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("temporary file left behind: %v", entries)
	}
	if err := writeTo(io.Discard, target, func(w io.Writer) error {
		_, werr := io.WriteString(w, "fresh")
		return werr
	}); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(target); string(body) != "fresh" {
		t.Errorf("target = %q, want fresh", body)
	}
}
