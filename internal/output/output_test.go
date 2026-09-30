package output

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

func cycleSnapshot() *model.Snapshot {
	return &model.Snapshot{
		Module: "m",
		Packages: []model.Package{
			{Path: "m/a", Imports: []string{"m/b"}, ImportedBy: []string{"m/b"}, Zone: model.ZoneMainSequence},
			{Path: "m/b", Imports: []string{"m/a", "m/c"}, ImportedBy: []string{"m/a"}, Zone: model.ZonePain},
			{Path: "m/c", ImportedBy: []string{"m/b"}, Zone: model.ZoneMainSequence},
		},
		Cycles:      [][]string{{"m/a", "m/b"}},
		Diagnostics: []model.Diagnostic{{ID: "dependency-cycle", Severity: model.SeverityError, Package: "m/a", Message: "import cycle"}},
	}
}

func render(t *testing.T, fn func(*bytes.Buffer, *model.Snapshot) error, snap *model.Snapshot) string {
	t.Helper()
	var b bytes.Buffer
	if err := fn(&b, snap); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestDotStylesCycleEdges(t *testing.T) {
	out := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Dot(b, s) }, cycleSnapshot())
	for _, want := range []string{
		`n1 -> n2 [color="red", penwidth=2];`,
		`n2 -> n1 [color="red", penwidth=2];`,
		"n2 -> n3;\n",
		`n2 [label="b", fillcolor="#f4b6b6"];`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dot output missing %q:\n%s", want, out)
		}
	}
}

func TestMermaidStylesCycleEdges(t *testing.T) {
	out := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Mermaid(b, s) }, cycleSnapshot())
	for _, want := range []string{"n1 --> n2", "linkStyle 0 stroke:#d00", "linkStyle 1 stroke:#d00", `n2["b"]:::pain`, "classDef pain"} {
		if !strings.Contains(out, want) {
			t.Errorf("mermaid output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "linkStyle 2") {
		t.Errorf("non-cycle edge styled:\n%s", out)
	}
}

func TestLabelQuoting(t *testing.T) {
	snap := &model.Snapshot{Module: "m", Packages: []model.Package{{Path: `m/we"ird`, Zone: model.ZoneIsolated}}}
	if out := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Dot(b, s) }, snap); !strings.Contains(out, `we\"ird`) {
		t.Errorf("label not escaped:\n%s", out)
	}
}

func TestMarkdownReducedGraphScope(t *testing.T) {
	snap := cycleSnapshot()
	snap.Packages = append(snap.Packages, model.Package{Path: "m/far", Zone: model.ZoneMainSequence})
	out := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Markdown(b, s) }, snap)
	if !strings.Contains(out, "<details>") || !strings.Contains(out, "Reduced graph: 3 packages, 3 edges") {
		t.Errorf("reduced graph missing or wrong scope:\n%s", out)
	}
	if strings.Contains(out, `"far"`) {
		t.Errorf("unrelated package leaked into the reduced graph:\n%s", out)
	}
}

func TestMarkdownOmitsGraph(t *testing.T) {
	clean := &model.Snapshot{Module: "m", Packages: []model.Package{{Path: "m/a", Zone: model.ZoneMainSequence}}}
	out := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Markdown(b, s) }, clean)
	if !strings.Contains(out, "graph omitted") || !strings.Contains(out, "None.") {
		t.Errorf("clean snapshot output:\n%s", out)
	}

	big := &model.Snapshot{Module: "m"}
	const n = 30
	for i := range n {
		p := model.Package{Path: fmt.Sprintf("m/p%02d", i), Zone: model.ZoneMainSequence}
		for j := range n {
			if i != j {
				p.Imports = append(p.Imports, fmt.Sprintf("m/p%02d", j))
			}
		}
		big.Packages = append(big.Packages, p)
	}
	big.Diagnostics = []model.Diagnostic{{ID: "god-package", Package: "m/p00", Severity: model.SeverityWarning}}
	out = render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Markdown(b, s) }, big)
	if !strings.Contains(out, "Graph omitted: 870 edges") {
		t.Errorf("oversized graph not omitted:\n%s", out[len(out)-300:])
	}
}

func TestCSVFormat(t *testing.T) {
	snap := &model.Snapshot{Packages: []model.Package{
		{Path: "m/b", Ca: 1, Instability: 2.0 / 3.0, Zone: model.ZonePain},
		{Path: "m/a"},
	}}
	out := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return CSV(b, s) }, snap)
	want := "package,ca,ce,i,na,nc,a,d,zone\nm/a,0,0,0.0000,0,0,0.0000,0.0000,\nm/b,1,0,0.6667,0,0,0.0000,0.0000,pain\n"
	if out != want {
		t.Errorf("csv = %q, want %q", out, want)
	}
}

func TestHistoryCSVSkipsFailedCommits(t *testing.T) {
	c1 := model.Commit{SHA: "aaa", Date: "2026-01-01T00:00:00Z"}
	c2 := model.Commit{SHA: "bbb", Date: "2026-01-02T00:00:00Z"}
	h := &model.History{Snapshots: []model.Entry{
		{Commit: c1, Snapshot: &model.Snapshot{Packages: []model.Package{{Path: "m/a", Ca: 1, Instability: 0.5, Zone: model.ZonePain}}}},
		{Commit: c2, Error: "boom"},
	}}
	var b bytes.Buffer
	if err := HistoryCSV(&b, h); err != nil {
		t.Fatal(err)
	}
	want := "sha,date,package,ca,ce,i,na,nc,a,d,zone\naaa,2026-01-01T00:00:00Z,m/a,1,0,0.5000,0,0,0.0000,0.0000,pain\n"
	if b.String() != want {
		t.Errorf("csv = %q", b.String())
	}
}

func fullSnapshot() *model.Snapshot {
	s := cycleSnapshot()
	s.Summary = model.Summary{Packages: 3, AvgDistance: 0.4, Pain: 1, Cycles: 1}
	for i := range s.Packages {
		s.Packages[i].Instability = model.Ratio(0.5)
		s.Packages[i].Distance = model.Ratio(float64(i) / 10)
	}
	return s
}

func TestTableLayout(t *testing.T) {
	out := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Table(b, s) }, fullSnapshot())
	lines := strings.Split(out, "\n")
	if !strings.HasPrefix(lines[0], "PACKAGE") {
		t.Errorf("header = %q", lines[0])
	}
	if c, b, a := strings.Index(out, "m/c"), strings.Index(out, "m/b"), strings.Index(out, "m/a"); c > b || b > a {
		t.Errorf("rows must be sorted by distance descending:\n%s", out)
	}
	for _, want := range []string{"Summary: packages=3", "Cycle: m/a -> m/b", "Diagnostics (1)", "[error] dependency-cycle m/a: import cycle"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}

func TestTableTiesSortByPath(t *testing.T) {
	s := &model.Snapshot{Packages: []model.Package{{Path: "m/b"}, {Path: "m/a"}}}
	out := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Table(b, s) }, s)
	if strings.Index(out, "m/a") > strings.Index(out, "m/b") {
		t.Errorf("ties must sort by path:\n%s", out)
	}
}

func TestJSONShape(t *testing.T) {
	out := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return JSON(b, s) }, fullSnapshot())
	for _, want := range []string{`"schema_version"`, `"packages": [`, `"instability": 0.5`, `"cycles": [`} {
		if !strings.Contains(out, want) {
			t.Errorf("json missing %q", want)
		}
	}
	if strings.Contains(out, "Interfaces") || !strings.HasSuffix(out, "}\n") {
		t.Error("json must omit analysis side channels and end with a newline")
	}
}

func TestHistoryJSON(t *testing.T) {
	h := &model.History{SchemaVersion: "1", Module: "m", Snapshots: []model.Entry{{Commit: model.Commit{SHA: "a"}, Error: "boom"}}}
	var b bytes.Buffer
	if err := HistoryJSON(&b, h); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"error": "boom"`) || !strings.Contains(b.String(), `"snapshots"`) {
		t.Errorf("history json = %s", b.String())
	}
}

func TestWriteAnnotationsEscapesAndDowngrades(t *testing.T) {
	var b bytes.Buffer
	err := WriteAnnotations(&b, []Annotation{
		{Level: "error", Title: "a,b:c", Message: "100%\nnext\r"},
		{Level: "weird", Title: "t", Message: "m"},
		{Level: "notice", Title: "n", Message: "ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "::error title=a%2Cb%3Ac::100%25%0Anext%0D\n::warning title=t::m\n::notice title=n::ok\n"
	if b.String() != want {
		t.Errorf("annotations = %q, want %q", b.String(), want)
	}
}

func TestShortNameOfModuleRoot(t *testing.T) {
	if got := shortName("github.com/x/tool", "github.com/x/tool"); got != "tool" {
		t.Errorf("root label = %q", got)
	}
	if got := shortName("github.com/x/tool", "github.com/x/tool/internal/a"); got != "internal/a" {
		t.Errorf("label = %q", got)
	}
}

func TestGraphWritersOnEmptySnapshot(t *testing.T) {
	empty := &model.Snapshot{Module: "m"}
	if out := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Dot(b, s) }, empty); !strings.HasPrefix(out, "digraph gocouple {") || !strings.HasSuffix(out, "}\n") {
		t.Errorf("empty dot = %q", out)
	}
	if out := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Mermaid(b, s) }, empty); !strings.HasPrefix(out, "flowchart LR") {
		t.Errorf("empty mermaid = %q", out)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("disk full") }

func TestWritersReportWriteErrors(t *testing.T) {
	s := fullSnapshot()
	h := &model.History{Snapshots: []model.Entry{{Snapshot: s}}}
	for name, err := range map[string]error{
		"json":     JSON(failWriter{}, s),
		"table":    Table(failWriter{}, s),
		"csv":      CSV(failWriter{}, s),
		"markdown": Markdown(failWriter{}, s),
		"dot":      Dot(failWriter{}, s),
		"mermaid":  Mermaid(failWriter{}, s),
		"hjson":    HistoryJSON(failWriter{}, h),
		"hcsv":     HistoryCSV(failWriter{}, h),
		"annot":    WriteAnnotations(failWriter{}, []Annotation{{Level: "error"}}),
	} {
		if err == nil {
			t.Errorf("%s writer swallowed a write error", name)
		}
	}
}

func TestSuppressedListedWithReason(t *testing.T) {
	snap := cycleSnapshot()
	snap.Suppressed = []model.Suppressed{{ID: "pain-zone", Package: "m/gen", Message: "x", Reason: "generated code"}}
	table := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Table(b, s) }, snap)
	md := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Markdown(b, s) }, snap)
	for name, out := range map[string]string{"table": table, "markdown": md} {
		if !strings.Contains(out, "Suppressed") || !strings.Contains(out, "pain-zone") || !strings.Contains(out, "generated code") {
			t.Errorf("%s misses the suppressed entry:\n%s", name, out)
		}
	}
	clean := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Table(b, s) }, cycleSnapshot())
	if strings.Contains(clean, "Suppressed") {
		t.Errorf("table without suppressed entries must not print the section:\n%s", clean)
	}
}

func TestWarningsListed(t *testing.T) {
	snap := cycleSnapshot()
	snap.Warnings = []string{"ignore[0]: pain-zone on m/none silenced no finding (stale)"}
	table := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Table(b, s) }, snap)
	md := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Markdown(b, s) }, snap)
	for name, out := range map[string]string{"table": table, "markdown": md} {
		if !strings.Contains(out, "Warnings") || !strings.Contains(out, "silenced no finding") {
			t.Errorf("%s misses the warning:\n%s", name, out)
		}
	}
	clean := render(t, func(b *bytes.Buffer, s *model.Snapshot) error { return Markdown(b, s) }, cycleSnapshot())
	if strings.Contains(clean, "Warnings") {
		t.Errorf("markdown without warnings must not print the section:\n%s", clean)
	}
}
