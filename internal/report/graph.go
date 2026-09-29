package report

import (
	"sort"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

const (
	maxGraphNodes = 60
	maxGraphEdges = 400
	nodeW         = 168.0
	nodeH         = 22.0
	colGap        = 64.0
	rowGap        = 30.0
	graphTop      = 30.0
	graphLeft     = 12.0
	columns       = 5
)

type gnode struct {
	ID, Label, Zone string
	X, Y, TextX     string
	TextY           string
	W, H            string
	Neighbors       string
	Title           string
}

type gedge struct {
	A, B           string
	X1, Y1, X2, Y2 string
	Class          string
	Marker         string
}

type colTitle struct{ X, Text string }

type graphView struct {
	Omitted   string
	ViewBox   string
	Nodes     []gnode
	Edges     []gedge
	ColTitles []colTitle
}

func layoutGraph(s *model.Snapshot) graphView {
	pkgs := append([]model.Package(nil), s.Packages...)
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Path < pkgs[j].Path })
	edgeCount := 0
	for _, p := range pkgs {
		edgeCount += len(p.Imports)
	}
	if len(pkgs) == 0 {
		return graphView{Omitted: "No packages."}
	}
	if len(pkgs) > maxGraphNodes || edgeCount > maxGraphEdges {
		return graphView{Omitted: "Graph omitted: " + itoa(len(pkgs)) + " packages and " + itoa(edgeCount) +
			" edges exceed the drawing budget. Use `gocouple analyze --format dot` or `--format mermaid`."}
	}

	inCycle := map[string]int{}
	for i, c := range s.Cycles {
		for _, p := range c {
			inCycle[p] = i + 1
		}
	}
	sdp := map[[2]string]bool{}
	for _, d := range s.Diagnostics {
		if d.ID != "sdp-violation" {
			continue
		}
		from, _ := d.Evidence["from"].(string)
		to, _ := d.Evidence["to"].(string)
		sdp[[2]string{from, to}] = true
	}

	col := func(p model.Package) int {
		return columns - 1 - min(int(float64(p.Instability)*columns), columns-1)
	}
	rowIdx := make([]int, columns)
	ids := map[string]string{}
	pos := map[string][2]float64{}
	colOf := map[string]int{}
	var g graphView
	maxRow := 0
	for i, p := range pkgs {
		c := col(p)
		r := rowIdx[c]
		rowIdx[c]++
		maxRow = max(maxRow, r+1)
		id := "g" + itoa(i+1)
		ids[p.Path] = id
		x := graphLeft + float64(c)*(nodeW+colGap)
		y := graphTop + float64(r)*rowGap
		pos[p.Path] = [2]float64{x, y}
		colOf[p.Path] = c
		g.Nodes = append(g.Nodes, gnode{
			ID: id, Label: truncate(shortPath(s.Module, p.Path), 26), Zone: p.Zone,
			X: f1(x), Y: f1(y), TextX: f1(x + 8), TextY: f1(y + 15), W: f1(nodeW), H: f1(nodeH),
			Title: shortPath(s.Module, p.Path) + " | I " + f2(float64(p.Instability)) + ", A " + f2(float64(p.Abstractness)) + ", " + p.Zone,
		})
	}

	nb := map[string][]string{}
	for _, p := range pkgs {
		for _, imp := range p.Imports {
			to, ok := ids[imp]
			if !ok {
				continue
			}
			from := ids[p.Path]
			nb[from] = append(nb[from], to)
			nb[to] = append(nb[to], from)
			e := gedge{A: from, B: to, Class: "edge", Marker: "arrow"}
			switch {
			case inCycle[p.Path] != 0 && inCycle[p.Path] == inCycle[imp]:
				e.Class, e.Marker = "edge edge-cycle", "arrow-cycle"
			case sdp[[2]string{p.Path, imp}]:
				e.Class, e.Marker = "edge edge-sdp", "arrow-sdp"
			}
			fx, fy, tx, ty := endpoints(pos[p.Path], colOf[p.Path], pos[imp], colOf[imp])
			e.X1, e.Y1, e.X2, e.Y2 = f1(fx), f1(fy), f1(tx), f1(ty)
			g.Edges = append(g.Edges, e)
		}
	}
	for i := range g.Nodes {
		list := nb[g.Nodes[i].ID]
		sort.Strings(list)
		g.Nodes[i].Neighbors = strings.Join(list, " ")
	}
	titles := []string{"I ≥ 0.8", "I 0.6–0.8", "I 0.4–0.6", "I 0.2–0.4", "I < 0.2"}
	for c := range columns {
		g.ColTitles = append(g.ColTitles, colTitle{X: f1(graphLeft + float64(c)*(nodeW+colGap)), Text: titles[c]})
	}
	width := graphLeft*2 + columns*nodeW + (columns-1)*colGap
	height := graphTop + float64(maxRow)*rowGap + 10
	g.ViewBox = "0 0 " + f1(width) + " " + f1(height)
	return g
}

func endpoints(from [2]float64, fromCol int, to [2]float64, toCol int) (fx, fy, tx, ty float64) {
	fy = from[1] + nodeH/2
	ty = to[1] + nodeH/2
	switch {
	case toCol > fromCol:
		return from[0] + nodeW, fy, to[0], ty
	case toCol < fromCol:
		return from[0], fy, to[0] + nodeW, ty
	default:
		return from[0] + nodeW, fy, to[0] + nodeW, ty
	}
}
