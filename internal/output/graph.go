package output

import (
	"fmt"
	"sort"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

type gnode struct {
	id    string
	label string
	zone  string
}

type gedge struct {
	from, to string
	cycle    bool
}

type depGraph struct {
	nodes []gnode
	edges []gedge
}

// buildGraph returns the internal dependency graph of the snapshot,
// restricted to keep when it is non-nil. Node ids follow path order.
func buildGraph(snap *model.Snapshot, keep map[string]bool) depGraph {
	inCycle := map[string]int{}
	for i, c := range snap.Cycles {
		for _, p := range c {
			inCycle[p] = i + 1
		}
	}
	pkgs := make([]model.Package, 0, len(snap.Packages))
	for _, p := range snap.Packages {
		if keep == nil || keep[p.Path] {
			pkgs = append(pkgs, p)
		}
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Path < pkgs[j].Path })

	ids := make(map[string]string, len(pkgs))
	var g depGraph
	for i, p := range pkgs {
		id := fmt.Sprintf("n%d", i+1)
		ids[p.Path] = id
		g.nodes = append(g.nodes, gnode{id: id, label: shortName(snap.Module, p.Path), zone: p.Zone})
	}
	for _, p := range pkgs {
		for _, imp := range p.Imports {
			to, ok := ids[imp]
			if !ok {
				continue
			}
			c := inCycle[p.Path]
			g.edges = append(g.edges, gedge{from: ids[p.Path], to: to, cycle: c != 0 && c == inCycle[imp]})
		}
	}
	return g
}

func shortName(module, path string) string {
	if path == module {
		return module[strings.LastIndex(module, "/")+1:]
	}
	return strings.TrimPrefix(path, module+"/")
}

func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

var zoneColor = map[string]string{
	model.ZonePain:         "#f4b6b6",
	model.ZoneUselessness:  "#f7d9a8",
	model.ZoneMainSequence: "#c9e7c9",
	model.ZoneIsolated:     "#dddddd",
}
