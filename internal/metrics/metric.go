package metrics

import (
	"math"
	"slices"
	"sort"

	"github.com/reinaldosaraiva/gocouple/internal/graph"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// Result is the pure outcome of measuring a set of loaded packages.
type Result struct {
	Packages []model.Package
	Cycles   [][]string
	Summary  model.Summary
}

// Instability returns Ce / (Ca + Ce), or 0 when both are zero.
func Instability(ca, ce int) float64 {
	if ca+ce == 0 {
		return 0
	}
	return float64(ce) / float64(ca+ce)
}

// Abstractness returns Na / Nc, or 0 when the package declares no types.
func Abstractness(na, nc int) float64 {
	if nc == 0 {
		return 0
	}
	return float64(na) / float64(nc)
}

// Distance returns |A + I - 1|.
func Distance(a, i float64) float64 {
	return math.Abs(a + i - 1)
}

// Zone classifies a package. A distance exactly at the threshold is on the
// main sequence.
func Zone(ca, ce int, a, i, threshold float64) string {
	if ca+ce == 0 {
		return model.ZoneIsolated
	}
	switch d := Distance(a, i); {
	case d <= threshold:
		return model.ZoneMainSequence
	case a+i < 1:
		return model.ZonePain
	default:
		return model.ZoneUselessness
	}
}

// Analyze fills the derived fields of every package and detects cycles.
// Input packages carry Path, IsMain, Imports, Na and Nc; imports that do not
// name another input package count toward Ce but not toward Ca.
func Analyze(pkgs []model.Package, threshold float64) Result {
	pkgs = uniqueByPath(pkgs)
	nodes := make([]string, len(pkgs))
	edges := make(map[string][]string, len(pkgs))
	for i, p := range pkgs {
		nodes[i] = p.Path
		edges[p.Path] = p.Imports
	}
	g := graph.New(nodes, edges)

	out := make([]model.Package, len(pkgs))
	for i, p := range pkgs {
		imports := slices.Clone(p.Imports)
		sort.Strings(imports)
		imports = slices.Compact(imports)
		if imports == nil {
			imports = []string{}
		}
		p.Imports = imports
		p.ImportedBy = g.ImportedBy(p.Path)
		p.Ca = len(p.ImportedBy)
		p.Ce = len(imports)
		i0 := Instability(p.Ca, p.Ce)
		a := Abstractness(p.Na, p.Nc)
		p.Instability = model.Ratio(i0)
		p.Abstractness = model.Ratio(a)
		p.Distance = model.Ratio(Distance(a, i0))
		p.Zone = Zone(p.Ca, p.Ce, a, i0, threshold)
		out[i] = p
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })

	cycles := g.Cycles()
	if cycles == nil {
		cycles = [][]string{}
	}
	return Result{Packages: out, Cycles: cycles, Summary: summarize(out, len(cycles))}
}

func summarize(pkgs []model.Package, cycles int) model.Summary {
	s := model.Summary{Packages: len(pkgs), Cycles: cycles}
	var total float64
	for _, p := range pkgs {
		total += float64(p.Distance)
		switch p.Zone {
		case model.ZonePain:
			s.Pain++
		case model.ZoneUselessness:
			s.Uselessness++
		case model.ZoneMainSequence:
			s.MainSequence++
		case model.ZoneIsolated:
			s.Isolated++
		}
	}
	if len(pkgs) > 0 {
		s.AvgDistance = model.Ratio(total / float64(len(pkgs)))
	}
	return s
}

func uniqueByPath(pkgs []model.Package) []model.Package {
	seen := make(map[string]bool, len(pkgs))
	out := make([]model.Package, 0, len(pkgs))
	for _, p := range pkgs {
		if seen[p.Path] {
			continue
		}
		seen[p.Path] = true
		out = append(out, p)
	}
	return out
}
