package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// Mermaid writes the internal dependency graph as a Mermaid flowchart.
func Mermaid(w io.Writer, snap *model.Snapshot) error {
	if _, err := io.WriteString(w, mermaidText(buildGraph(snap, nil))); err != nil {
		return fmt.Errorf("writing mermaid: %w", err)
	}
	return nil
}

func mermaidText(g depGraph) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	for _, n := range g.nodes {
		fmt.Fprintf(&b, "  %s[%s]:::%s\n", n.id, quote(n.label), n.zone)
	}
	var cycleLinks []int
	for i, e := range g.edges {
		fmt.Fprintf(&b, "  %s --> %s\n", e.from, e.to)
		if e.cycle {
			cycleLinks = append(cycleLinks, i)
		}
	}
	for _, i := range cycleLinks {
		fmt.Fprintf(&b, "  linkStyle %d stroke:#d00,stroke-width:2px\n", i)
	}
	for _, zone := range []string{model.ZoneIsolated, model.ZoneMainSequence, model.ZonePain, model.ZoneUselessness} {
		fmt.Fprintf(&b, "  classDef %s fill:%s,stroke:#555\n", zone, zoneColor[zone])
	}
	return b.String()
}
