package report

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/diagnose"
	"github.com/reinaldosaraiva/gocouple/internal/metrics"
	"github.com/reinaldosaraiva/gocouple/internal/model"
	"golang.org/x/net/html"
)

var update = flag.Bool("update", false, "rewrite golden files")

func snapshot(module string, pkgs []model.Package) *model.Snapshot {
	cfg := config.Default()
	res := metrics.Analyze(pkgs, cfg.DistanceThreshold)
	snap := &model.Snapshot{
		SchemaVersion: model.SchemaVersion, ToolVersion: "test", Module: module,
		Config:   model.Config{DistanceThreshold: cfg.DistanceThreshold},
		Packages: res.Packages, Cycles: res.Cycles, Summary: res.Summary,
	}
	snap.Diagnostics = diagnose.Run(context.Background(), snap, cfg, diagnose.Rules())
	return snap
}

const mod = "example.com/demo"

func pkg(name string, na, nc int, imports ...string) model.Package {
	full := make([]string, len(imports))
	for i, imp := range imports {
		full[i] = mod + "/" + imp
	}
	return model.Package{Path: mod + "/" + name, Na: na, Nc: nc, Imports: full}
}

func demoHistory() *model.History {
	stage1 := []model.Package{pkg("api", 0, 1, "core"), pkg("core", 1, 2)}
	stage2 := []model.Package{
		pkg("api", 0, 1, "core", "log", "db", "cache", "mail", "auth", "queue", "search", "billing"),
		pkg("core", 1, 2, "log"), pkg("log", 0, 1), pkg("db", 0, 2, "log"), pkg("cache", 0, 1, "log"),
		pkg("mail", 0, 1, "log"), pkg("auth", 0, 1, "log", "db"), pkg("queue", 0, 1, "log"),
		pkg("search", 0, 1, "log", "billing"), pkg("billing", 0, 1, "search"),
	}
	stage3 := []model.Package{
		pkg("api", 0, 1, "core"), pkg("core", 2, 3), pkg("db", 1, 2, "core"), pkg("auth", 0, 1, "core"),
	}
	h := &model.History{SchemaVersion: model.SchemaVersion, Module: mod}
	for i, pk := range [][]model.Package{stage1, stage2, stage3} {
		subjects := []string{"feat: add api and core packages", "feat: add integrations, logging and billing search", "refactor: split integrations and remove the cycle"}
		shas := []string{"3f9a1c2d4e5b6a7980c1d2e3f4a5b6c7d8e9f001", "b81d47e0a3c592f6d10e8b7a4c3f2e1d0a9b8c72", "9c02e5f7a1b34d68e0f1a2b3c4d5e6f708192a3b"}
		c := model.Commit{SHA: shas[i], Date: fmt.Sprintf("2026-03-0%dT12:00:00Z", i+1), Subject: subjects[i]}
		s := snapshot(mod, pk)
		s.Commit = &c
		h.Snapshots = append(h.Snapshots, model.Entry{Commit: c, Snapshot: s})
	}
	h.Snapshots = append(h.Snapshots[:2], append([]model.Entry{{
		Commit: model.Commit{SHA: "deadbeefdeadbeef", Date: "2026-03-05T12:00:00Z", Subject: "broken"}, Error: "does not compile\nsecond line",
	}}, h.Snapshots[2:]...)...)
	return h
}

func render(t *testing.T, in Input) string {
	t.Helper()
	var b bytes.Buffer
	if err := Render(&b, in); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden (run with -update): %v", err)
	}
	if string(want) != got {
		t.Errorf("output differs from %s (run with -update to accept)", name)
	}
}

func TestGoldenAndDeterminism(t *testing.T) {
	h := demoHistory()
	first := render(t, Input{History: h})
	if second := render(t, Input{History: h}); first != second {
		t.Fatal("history report is not deterministic")
	}
	golden(t, "history.golden.html", first)

	snap := snapshot(mod, []model.Package{pkg("a", 0, 1, "b"), pkg("b", 1, 2)})
	golden(t, "snapshot.golden.html", render(t, Input{Snapshot: snap}))
}

func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func TestSelfContainedAndStructure(t *testing.T) {
	out := render(t, Input{History: demoHistory()})
	if strings.Contains(out, "http://") || strings.Contains(out, "https://") {
		t.Error("report references an external URL")
	}
	doc, err := html.Parse(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	svgs, links, scriptSrc, frames := 0, 0, 0, map[string]bool{}
	walk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		switch n.Data {
		case "svg":
			svgs++
		case "link":
			links++
		case "script":
			if _, ok := attr(n, "src"); ok {
				scriptSrc++
			}
		}
		for _, a := range n.Attr {
			if (a.Key == "src" || a.Key == "href" || a.Key == "srcset") && !strings.HasPrefix(a.Val, "#") {
				t.Errorf("external reference %s=%q on <%s>", a.Key, a.Val, n.Data)
			}
		}
		if v, ok := attr(n, "data-frame"); ok && n.Data == "p" {
			frames[v] = true
		}
	})
	// defs + A x I chart + 4 series + one dependency graph per frame
	if want := 1 + 1 + 4 + 3; svgs != want {
		t.Errorf("svg count = %d, want %d", svgs, want)
	}
	if links != 0 || scriptSrc != 0 {
		t.Errorf("links=%d script src=%d, want none", links, scriptSrc)
	}
	if len(frames) != 3 {
		t.Errorf("frames = %v, want 3 (failed commit skipped)", frames)
	}
	for _, want := range []string{"Skipped commits", "deadbee: does not compile", "edge edge-cycle", `id="frame"`, "prefers-color-scheme: dark", "viewport"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(out, "second line") {
		t.Error("skipped commit error must keep only its first line")
	}
}

func TestSnapshotOnlyHasNoHistoryControls(t *testing.T) {
	out := render(t, Input{Snapshot: snapshot(mod, []model.Package{pkg("a", 0, 1)})})
	if strings.Contains(out, `id="frame"`) || strings.Contains(out, "Over time") {
		t.Error("snapshot-only report must not show the slider or series")
	}
	if !strings.Contains(out, "current analysis") {
		t.Error("frame label missing")
	}
}

func TestSnapshotAppendedToHistory(t *testing.T) {
	out := render(t, Input{History: demoHistory(), Snapshot: snapshot(mod, []model.Package{pkg("a", 0, 1)})})
	if !strings.Contains(out, `max="3"`) || !strings.Contains(out, "current analysis") {
		t.Error("snapshot must be the last frame of the history")
	}
}

func TestEscapesUserContent(t *testing.T) {
	evil := `<script>alert(1)</script>"><img src=x>`
	s := snapshot(mod, []model.Package{{Path: mod + "/x", Nc: 1, Imports: []string{mod + "/" + evil}}, {Path: mod + "/" + evil, Nc: 1}})
	s.Diagnostics = append(s.Diagnostics, model.Diagnostic{ID: "x", Severity: "warning", Package: mod + "/x", Message: evil})
	out := render(t, Input{Snapshot: s})
	if strings.Contains(out, "<script>alert(1)") || strings.Contains(out, "<img src=x>") {
		t.Error("user content reached the page unescaped")
	}
}

func TestErrorsWithoutInput(t *testing.T) {
	if err := Render(&bytes.Buffer{}, Input{}); err == nil {
		t.Error("expected error for empty input")
	}
	failed := &model.History{Snapshots: []model.Entry{{Error: "boom"}}}
	if err := Render(&bytes.Buffer{}, Input{History: failed}); err == nil {
		t.Error("expected error when every commit failed")
	}
}

func TestLargeGraphIsOmitted(t *testing.T) {
	var pkgs []model.Package
	for i := range maxGraphNodes + 1 {
		pkgs = append(pkgs, pkg(fmt.Sprintf("p%02d", i), 0, 1))
	}
	out := render(t, Input{Snapshot: snapshot(mod, pkgs)})
	if !strings.Contains(out, "Graph omitted") || strings.Contains(out, "<svg class=\"dep-graph\"") {
		t.Error("oversized graph must be replaced by a note")
	}
}

func TestSDPEdgeIsHighlighted(t *testing.T) {
	s := snapshot(mod, []model.Package{
		pkg("stable", 0, 1, "volatile"),
		pkg("volatile", 0, 1, "x1", "x2", "x3"),
		pkg("x1", 0, 1), pkg("x2", 0, 1), pkg("x3", 0, 1),
		pkg("user", 0, 1, "stable"),
	})
	if out := render(t, Input{Snapshot: s}); !strings.Contains(out, "edge edge-sdp") {
		t.Error("sdp-violation edge not highlighted")
	}
}

var cssVar = regexp.MustCompile(`--([a-z-]+):\s*(#[0-9a-fA-F]{6});`)

func luminance(hex string) float64 {
	c := func(s string) float64 {
		v, _ := strconv.ParseUint(s, 16, 8)
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*c(hex[1:3]) + 0.7152*c(hex[3:5]) + 0.0722*c(hex[5:7])
}

func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

func TestPaletteContrast(t *testing.T) {
	light, dark, ok := strings.Cut(reportCSS, "@media (prefers-color-scheme: dark)")
	if !ok {
		t.Fatal("dark palette missing")
	}
	dark, _, _ = strings.Cut(dark, "* { box-sizing")
	for name, block := range map[string]string{"light": light, "dark": dark} {
		vars := map[string]string{}
		for _, m := range cssVar.FindAllStringSubmatch(block, -1) {
			vars[m[1]] = m[2]
		}
		checks := []struct {
			fg, bg string
			min    float64
		}{
			{"fg", "bg", 7}, {"fg", "card", 7}, {"muted", "bg", 4.5}, {"muted", "card", 4.5}, {"accent", "card", 4.5},
			{"pain", "card", 4.5}, {"use", "card", 4.5}, {"ok", "card", 4.5}, {"iso", "card", 3},
		}
		for _, c := range checks {
			if got := contrast(vars[c.fg], vars[c.bg]); got < c.min {
				t.Errorf("%s theme: %s on %s contrast %.2f < %.1f", name, c.fg, c.bg, got, c.min)
			}
		}
	}
}

func TestSuppressedAndWarningsRendered(t *testing.T) {
	evil := `<b>reason</b>`
	s := snapshot(mod, []model.Package{{Path: mod + "/x", Nc: 1}})
	s.Suppressed = []model.Suppressed{{ID: "pain-zone", Package: mod + "/gen", Message: "in pain", Reason: evil}}
	s.Warnings = []string{"ignore[0]: god-package on **/none silenced no finding (stale)"}
	out := render(t, Input{Snapshot: s})
	for _, want := range []string{"<h3>Suppressed</h3>", "sev-suppressed", "<code>gen</code>", "&lt;b&gt;reason&lt;/b&gt;", "<h3>Warnings</h3>", "silenced no finding"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(out, evil) {
		t.Error("reason reached the page unescaped")
	}
	clean := render(t, Input{Snapshot: snapshot(mod, []model.Package{{Path: mod + "/x", Nc: 1}})})
	if strings.Contains(clean, "<h3>Suppressed</h3>") || strings.Contains(clean, "<h3>Warnings</h3>") {
		t.Error("sections must be absent without entries")
	}
}
