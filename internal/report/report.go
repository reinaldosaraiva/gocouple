package report

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

//go:embed assets/report.css
var reportCSS string

//go:embed assets/report.js
var reportJS string

//go:embed assets/report.html.tmpl
var reportTemplate string

// Input is what the report renders: a history, a single snapshot, or both
// (the snapshot is then appended as the latest frame).
type Input struct {
	History  *model.History
	Snapshot *model.Snapshot
}

// Render writes a self-contained HTML document. The same input always yields
// the same bytes.
func Render(w io.Writer, in Input) error {
	v, err := buildView(in)
	if err != nil {
		return err
	}
	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("parsing report template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return fmt.Errorf("rendering report: %w", err)
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	return nil
}

type view struct {
	Title      string
	Module     string
	CSS        template.CSS
	JS         template.JS
	Frames     []frame
	Last       int
	HasHistory bool
	Skipped    []string
	Chart      chartGeom
	Trails     []trail
	TrailPkgs  []string
	Series     []series
}

type frame struct {
	Index      int
	Label      string
	Hidden     bool
	Summary    model.Summary
	AvgD       string
	Points     []point
	Graph      graphView
	Rows       []row
	Churn      bool
	Diags      []diagRow
	Suppressed []suppressedRow
	Warnings   []string
}

type row struct {
	Path             string
	Ca, Ce, Na, Nc   int
	ChurnN           int
	I, A, D          string
	IRaw, ARaw, DRaw string
	Zone             string
}

type diagRow struct {
	Severity, ID, Package, Message string
}

type suppressedRow struct {
	ID, Package, Message, Reason string
}

func buildView(in Input) (view, error) {
	var snaps []*model.Snapshot
	var labels []string
	var skipped []string
	module := ""
	if in.History != nil {
		module = in.History.Module
		for _, e := range in.History.Snapshots {
			if e.Snapshot == nil {
				skipped = append(skipped, fmt.Sprintf("%s: %s", shortSHA(e.Commit.SHA), firstLine(e.Error)))
				continue
			}
			snaps = append(snaps, e.Snapshot)
			labels = append(labels, commitLabel(e.Commit))
		}
	}
	if in.Snapshot != nil {
		snaps = append(snaps, in.Snapshot)
		labels = append(labels, "current analysis")
		if module == "" {
			module = in.Snapshot.Module
		}
	}
	if len(snaps) == 0 {
		return view{}, errors.New("report needs at least one snapshot (history with a successful commit, or a snapshot)")
	}

	threshold := snaps[len(snaps)-1].Config.DistanceThreshold
	v := view{
		Title:      "gocouple report: " + module,
		Module:     module,
		CSS:        template.CSS(reportCSS),
		JS:         template.JS(reportJS),
		Last:       len(snaps) - 1,
		HasHistory: len(snaps) > 1,
		Skipped:    skipped,
		Chart:      newChartGeom(threshold),
	}
	for i, s := range snaps {
		f := frame{
			Index:      i,
			Label:      labels[i],
			Hidden:     i != len(snaps)-1,
			Summary:    s.Summary,
			AvgD:       f2(float64(s.Summary.AvgDistance)),
			Points:     points(s, v.Chart),
			Graph:      layoutGraph(s),
			Rows:       rows(s),
			Churn:      s.Config.VolatilitySince != "",
			Diags:      diags(s),
			Suppressed: suppressed(s),
			Warnings:   s.Warnings,
		}
		v.Frames = append(v.Frames, f)
	}
	v.Trails, v.TrailPkgs = trails(snaps, v.Chart)
	if v.HasHistory {
		v.Series = buildSeries(snaps, labels)
	}
	return v, nil
}

func commitLabel(c model.Commit) string {
	date, _, _ := strings.Cut(c.Date, "T")
	label := shortSHA(c.SHA) + " " + date
	if c.Subject != "" {
		label += " " + truncate(c.Subject, 60)
	}
	return label
}

func rows(s *model.Snapshot) []row {
	pkgs := append([]model.Package(nil), s.Packages...)
	sort.SliceStable(pkgs, func(i, j int) bool {
		if pkgs[i].Distance != pkgs[j].Distance {
			return pkgs[i].Distance > pkgs[j].Distance
		}
		return pkgs[i].Path < pkgs[j].Path
	})
	out := make([]row, len(pkgs))
	for i, p := range pkgs {
		out[i] = row{
			Path: shortPath(s.Module, p.Path), Ca: p.Ca, Ce: p.Ce, Na: p.Na, Nc: p.Nc,
			I: f2(float64(p.Instability)), A: f2(float64(p.Abstractness)), D: f2(float64(p.Distance)),
			IRaw: f4(float64(p.Instability)), ARaw: f4(float64(p.Abstractness)), DRaw: f4(float64(p.Distance)),
			Zone: p.Zone, ChurnN: p.Churn,
		}
	}
	return out
}

func diags(s *model.Snapshot) []diagRow {
	out := make([]diagRow, len(s.Diagnostics))
	for i, d := range s.Diagnostics {
		out[i] = diagRow{Severity: d.Severity, ID: d.ID, Package: shortPath(s.Module, d.Package), Message: d.Message}
	}
	return out
}

func suppressed(s *model.Snapshot) []suppressedRow {
	out := make([]suppressedRow, len(s.Suppressed))
	for i, d := range s.Suppressed {
		out[i] = suppressedRow{ID: d.ID, Package: shortPath(s.Module, d.Package), Message: d.Message, Reason: d.Reason}
	}
	return out
}

func shortPath(module, path string) string {
	if path == module {
		return module[strings.LastIndex(module, "/")+1:]
	}
	return strings.TrimPrefix(path, module+"/")
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return truncate(line, 140)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
func f4(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }
