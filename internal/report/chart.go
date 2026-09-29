package report

import (
	"math"
	"sort"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

const (
	chartW    = 560.0
	chartH    = 500.0
	chartLeft = 56.0
	chartTop  = 20.0
	plotW     = 480.0
	plotH     = 420.0
)

type tick struct {
	Label  string
	X, Y   string
	Pos    string
	Length string
}

type chartGeom struct {
	ViewBox        string
	Left, Top      string
	Width, Height  string
	Right, Bottom  string
	PainPoints     string
	UselessPoints  string
	MainSeqX1      string
	MainSeqY1      string
	MainSeqX2      string
	MainSeqY2      string
	XTicks, YTicks []tick
	XTitleX        string
	XTitleY        string
	YTitleX        string
	YTitleY        string
	Threshold      string
}

type point struct {
	CX, CY, R string
	Zone      string
	Title     string
}

type trail struct {
	Pkg    string
	Points string
	Dots   []dot
}

type dot struct{ CX, CY string }

func px(i float64) float64 { return chartLeft + i*plotW }
func py(a float64) float64 { return chartTop + (1-a)*plotH }

func newChartGeom(threshold float64) chartGeom {
	t := math.Min(math.Max(threshold, 0), 1)
	g := chartGeom{
		ViewBox: "0 0 " + f1(chartW) + " " + f1(chartH),
		Left:    f1(chartLeft), Top: f1(chartTop), Width: f1(plotW), Height: f1(plotH),
		Right: f1(chartLeft + plotW), Bottom: f1(chartTop + plotH),
		MainSeqX1: f1(px(0)), MainSeqY1: f1(py(1)), MainSeqX2: f1(px(1)), MainSeqY2: f1(py(0)),
		XTitleX: f1(chartLeft + plotW/2), XTitleY: f1(chartTop + plotH + 46),
		YTitleX: f1(16), YTitleY: f1(chartTop + plotH/2),
		Threshold: f2(threshold),
	}
	if t < 1 {
		g.PainPoints = poly([][2]float64{{0, 0}, {1 - t, 0}, {0, 1 - t}})
		g.UselessPoints = poly([][2]float64{{1, 1}, {t, 1}, {1, t}})
	}
	for _, v := range []float64{0, 0.25, 0.5, 0.75, 1} {
		label := f2(v)
		g.XTicks = append(g.XTicks, tick{Label: label, X: f1(px(v)), Y: f1(chartTop + plotH + 22), Pos: f1(px(v))})
		g.YTicks = append(g.YTicks, tick{Label: label, X: f1(chartLeft - 8), Y: f1(py(v) + 4), Pos: f1(py(v))})
	}
	return g
}

func poly(pts [][2]float64) string {
	parts := make([]string, len(pts))
	for i, p := range pts {
		parts[i] = f1(px(p[0])) + "," + f1(py(p[1]))
	}
	return strings.Join(parts, " ")
}

func points(s *model.Snapshot, _ chartGeom) []point {
	pkgs := append([]model.Package(nil), s.Packages...)
	sort.SliceStable(pkgs, func(i, j int) bool {
		if pkgs[i].Ca != pkgs[j].Ca {
			return pkgs[i].Ca > pkgs[j].Ca
		}
		return pkgs[i].Path < pkgs[j].Path
	})
	out := make([]point, len(pkgs))
	for i, p := range pkgs {
		r := math.Min(4+3*math.Sqrt(float64(p.Ca)), 18)
		out[i] = point{
			CX: f1(px(float64(p.Instability))), CY: f1(py(float64(p.Abstractness))), R: f1(r), Zone: p.Zone,
			Title: shortPath(s.Module, p.Path) + " | Ca " + itoa(p.Ca) + ", Ce " + itoa(p.Ce) +
				", I " + f2(float64(p.Instability)) + ", A " + f2(float64(p.Abstractness)) + ", D " + f2(float64(p.Distance)) + " | " + p.Zone,
		}
	}
	return out
}

func trails(snaps []*model.Snapshot, _ chartGeom) ([]trail, []string) {
	if len(snaps) < 2 {
		return nil, nil
	}
	type pos struct{ i, a float64 }
	series := map[string][]pos{}
	for _, s := range snaps {
		for _, p := range s.Packages {
			name := shortPath(s.Module, p.Path)
			series[name] = append(series[name], pos{float64(p.Instability), float64(p.Abstractness)})
		}
	}
	names := make([]string, 0, len(series))
	for n, ps := range series {
		if len(ps) >= 2 {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	out := make([]trail, len(names))
	for i, n := range names {
		var pts []string
		var dots []dot
		for _, p := range series[n] {
			x, y := f1(px(p.i)), f1(py(p.a))
			pts = append(pts, x+","+y)
			dots = append(dots, dot{CX: x, CY: y})
		}
		out[i] = trail{Pkg: n, Points: strings.Join(pts, " "), Dots: dots}
	}
	return out, names
}
