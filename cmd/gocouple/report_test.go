package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportFromSnapshot(t *testing.T) {
	dir := t.TempDir()
	snap := filepath.Join(dir, "analysis.json")
	if _, err := runCLI(t, "analyze", "--dir", fixtureDir("orders-legacy"), "--format", "json", "--out", snap); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "report.html")
	if _, err := runCLI(t, "report", "--snapshot", snap, "--out", out); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	for _, want := range []string{"<svg", "wasted-abstraction", "internal/logger", "example.com/orderslegacy"} {
		if !strings.Contains(html, want) {
			t.Errorf("report missing %q", want)
		}
	}
	if strings.Contains(html, "http://") || strings.Contains(html, "https://") {
		t.Error("report references an external URL")
	}
}

func TestReportErrors(t *testing.T) {
	if _, err := runCLI(t, "report"); err == nil {
		t.Error("expected error without inputs")
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"schema_version":"2"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "report", "--snapshot", bad, "--out", filepath.Join(dir, "r.html")); err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("err = %v", err)
	}
	if _, err := runCLI(t, "report", "--history", filepath.Join(dir, "missing.json")); err == nil {
		t.Error("expected error for a missing file")
	}
}
