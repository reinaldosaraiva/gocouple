package report

import (
	"math"
	"strconv"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

const (
	seriesW      = 320.0
	seriesH      = 150.0
	seriesLeft   = 40.0
	seriesRight  = 10.0
	seriesTop    = 14.0
	seriesBottom = 22.0
)

type line struct {
	Class  string
	Name   string
	Points string
}

type marker struct {
	Frame int
	X     string
}

type series struct {
	Title   string
	ViewBox string
	Lines   []line
	Markers []marker
	MaxLbl  string
	MinLbl  string
	MaxY    string
	MinY    string
	AxisX1  string
	AxisX2  string
	Legend  []line
	Left    string
	Top     string
	Base    string
	First   string
	Last    string
	LabelY  string
	MidX    string
}

func buildSeries(snaps []*model.Snapshot, labels []string) []series {
	n := len(snaps)
	avg := make([]float64, n)
	pain := make([]float64, n)
	cyc := make([]float64, n)
	var errs, warns, infos = make([]float64, n), make([]float64, n), make([]float64, n)
	for i, s := range snaps {
		avg[i] = float64(s.Summary.AvgDistance)
		pain[i] = float64(s.Summary.Pain)
		cyc[i] = float64(s.Summary.Cycles)
		for _, d := range s.Diagnostics {
			switch d.Severity {
			case model.SeverityError:
				errs[i]++
			case model.SeverityWarning:
				warns[i]++
			default:
				infos[i]++
			}
		}
	}
	return []series{
		newSeries("Average distance (D)", labels, []namedValues{{"series-line", "D", avg}}, true),
		newSeries("Packages in the pain zone", labels, []namedValues{{"series-line", "pain", pain}}, false),
		newSeries("Dependency cycles", labels, []namedValues{{"series-line", "cycles", cyc}}, false),
		newSeries("Diagnostics by severity", labels, []namedValues{
			{"series-line series-error", "error", errs},
			{"series-line series-warning", "warning", warns},
			{"series-line series-info", "info", infos},
		}, false),
	}
}

type namedValues struct {
	class, name string
	values      []float64
}

func newSeries(title string, labels []string, lines []namedValues, fixedUnit bool) series {
	n := len(labels)
	maxV := 0.0
	for _, l := range lines {
		for _, v := range l.values {
			maxV = math.Max(maxV, v)
		}
	}
	switch {
	case fixedUnit:
		maxV = math.Max(maxV, 0.1)
		maxV = math.Ceil(maxV*10) / 10
	case maxV < 1:
		maxV = 1
	}
	plotWd := seriesW - seriesLeft - seriesRight
	plotHt := seriesH - seriesTop - seriesBottom
	x := func(i int) float64 {
		if n == 1 {
			return seriesLeft + plotWd/2
		}
		return seriesLeft + plotWd*float64(i)/float64(n-1)
	}
	y := func(v float64) float64 { return seriesTop + plotHt*(1-v/maxV) }

	s := series{
		Title:   title,
		ViewBox: "0 0 " + f1(seriesW) + " " + f1(seriesH),
		MaxLbl:  trimNum(maxV), MinLbl: "0",
		MaxY:   f1(seriesTop + 4),
		MinY:   f1(seriesTop + plotHt),
		AxisX1: f1(seriesLeft), AxisX2: f1(seriesW - seriesRight),
		Left: f1(seriesLeft), Top: f1(seriesTop), Base: f1(seriesTop + plotHt),
		First: shortLabel(labels[0]), Last: shortLabel(labels[n-1]),
		LabelY: f1(seriesH - 6), MidX: f1(seriesLeft + plotWd/2),
	}
	for _, l := range lines {
		pts := make([]string, n)
		for i, v := range l.values {
			pts[i] = f1(x(i)) + "," + f1(y(v))
		}
		s.Lines = append(s.Lines, line{Class: l.class, Name: l.name, Points: strings.Join(pts, " ")})
	}
	if len(lines) > 1 {
		s.Legend = s.Lines
	}
	for i := range labels {
		s.Markers = append(s.Markers, marker{Frame: i, X: f1(x(i))})
	}
	return s
}

func trimNum(v float64) string {
	if v == math.Trunc(v) {
		return strconv.Itoa(int(v))
	}
	return f1(v)
}

func shortLabel(l string) string {
	fields := strings.Fields(l)
	if len(fields) >= 2 {
		return fields[0] + " " + fields[1]
	}
	return l
}

func itoa(i int) string { return strconv.Itoa(i) }
