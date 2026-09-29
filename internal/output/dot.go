package output

import (
	"fmt"
	"io"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// Dot writes the internal dependency graph in Graphviz DOT syntax. Nodes are
// colored by zone and edges inside a cycle are red.
func Dot(w io.Writer, snap *model.Snapshot) error {
	g := buildGraph(snap, nil)
	if _, err := fmt.Fprintf(w, "digraph gocouple {\n  rankdir=LR;\n  node [shape=box, style=filled];\n"); err != nil {
		return fmt.Errorf("writing dot: %w", err)
	}
	for _, n := range g.nodes {
		if _, err := fmt.Fprintf(w, "  %s [label=%s, fillcolor=%s];\n", n.id, quote(n.label), quote(zoneColor[n.zone])); err != nil {
			return fmt.Errorf("writing dot: %w", err)
		}
	}
	for _, e := range g.edges {
		attr := ""
		if e.cycle {
			attr = ` [color="red", penwidth=2]`
		}
		if _, err := fmt.Fprintf(w, "  %s -> %s%s;\n", e.from, e.to, attr); err != nil {
			return fmt.Errorf("writing dot: %w", err)
		}
	}
	if _, err := io.WriteString(w, "}\n"); err != nil {
		return fmt.Errorf("writing dot: %w", err)
	}
	return nil
}
