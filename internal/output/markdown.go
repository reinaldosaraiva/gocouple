package output

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

const (
	maxGraphEdges = 300
	maxGraphBytes = 40 * 1024
)

// Markdown writes a report ready to paste into a pull request: summary,
// package table, diagnostics and a reduced dependency graph.
func Markdown(w io.Writer, snap *model.Snapshot) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# gocouple: %s\n\n", snap.Module)

	s := snap.Summary
	b.WriteString("| Metric | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| Packages | %d |\n| Average distance | %.2f |\n| Pain | %d |\n| Uselessness | %d |\n| Main sequence | %d |\n| Isolated | %d |\n| Cycles | %d |\n\n",
		s.Packages, float64(s.AvgDistance), s.Pain, s.Uselessness, s.MainSequence, s.Isolated, s.Cycles)

	churn := snap.Config.VolatilitySince != ""
	b.WriteString("## Packages\n\n| Package | Ca | Ce | I | Na | Nc | A | D | Zone |")
	sep := "\n|---|---|---|---|---|---|---|---|---|"
	if churn {
		b.WriteString(" Churn |")
		sep += "---|"
	}
	b.WriteString(sep + "\n")
	for _, p := range sortedByDistance(snap.Packages) {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %.2f | %d | %d | %.2f | %.2f | %s |",
			p.Path, p.Ca, p.Ce, float64(p.Instability), p.Na, p.Nc, float64(p.Abstractness), float64(p.Distance), p.Zone)
		if churn {
			fmt.Fprintf(&b, " %d |", p.Churn)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n## Diagnostics\n\n")
	if len(snap.Diagnostics) == 0 {
		b.WriteString("None.\n")
	}
	for _, d := range snap.Diagnostics {
		fmt.Fprintf(&b, "- **%s** `%s` `%s`: %s\n", d.Severity, d.ID, d.Package, d.Message)
	}

	if len(snap.Suppressed) > 0 {
		b.WriteString("\n## Suppressed\n\n")
		for _, d := range snap.Suppressed {
			fmt.Fprintf(&b, "- `%s` `%s`: %s\n", d.ID, d.Package, d.Reason)
		}
	}

	if len(snap.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, m := range snap.Warnings {
			fmt.Fprintf(&b, "- %s\n", m)
		}
	}

	b.WriteString("\n## Dependency graph\n\n")
	b.WriteString(reducedGraph(snap))

	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("writing markdown: %w", err)
	}
	return nil
}

func sortedByDistance(pkgs []model.Package) []model.Package {
	out := slices.Clone(pkgs)
	slices.SortFunc(out, func(a, b model.Package) int {
		switch {
		case a.Distance > b.Distance:
			return -1
		case a.Distance < b.Distance:
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})
	return out
}

func reducedGraph(snap *model.Snapshot) string {
	focus := map[string]bool{}
	for _, c := range snap.Cycles {
		for _, p := range c {
			focus[p] = true
		}
	}
	for _, d := range snap.Diagnostics {
		focus[d.Package] = true
	}
	if len(focus) == 0 {
		return "No packages in cycles or with diagnostics; graph omitted.\n"
	}
	keep := map[string]bool{}
	for _, p := range snap.Packages {
		if focus[p.Path] {
			keep[p.Path] = true
			for _, n := range p.Imports {
				keep[n] = true
			}
			for _, n := range p.ImportedBy {
				keep[n] = true
			}
		}
	}
	g := buildGraph(snap, keep)
	text := mermaidText(g)
	if len(g.edges) > maxGraphEdges || len(text) > maxGraphBytes {
		return fmt.Sprintf("Graph omitted: %d edges exceed the rendering budget; run `gocouple analyze --format mermaid`.\n", len(g.edges))
	}
	return fmt.Sprintf("<details>\n<summary>Reduced graph: %d packages, %d edges</summary>\n\n```mermaid\n%s```\n\n</details>\n",
		len(g.nodes), len(g.edges), text)
}
