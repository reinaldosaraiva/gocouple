package output

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// Table writes packages sorted by distance descending then path, followed by
// the summary and the diagnostics.
func Table(w io.Writer, snap *model.Snapshot) error {
	pkgs := slices.Clone(snap.Packages)
	slices.SortFunc(pkgs, func(a, b model.Package) int {
		if a.Distance != b.Distance {
			if a.Distance > b.Distance {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PACKAGE\tCa\tCe\tI\tNa\tNc\tA\tD\tZONE")
	for _, p := range pkgs {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%d\t%.2f\t%d\t%d\t%.2f\t%.2f\t%s\n",
			p.Path, p.Ca, p.Ce, float64(p.Instability), p.Na, p.Nc,
			float64(p.Abstractness), float64(p.Distance), p.Zone)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("writing table: %w", err)
	}

	s := snap.Summary
	if _, err := fmt.Fprintf(w, "\nSummary: packages=%d avg_distance=%.2f pain=%d uselessness=%d main_sequence=%d isolated=%d cycles=%d\n",
		s.Packages, float64(s.AvgDistance), s.Pain, s.Uselessness, s.MainSequence, s.Isolated, s.Cycles); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}
	for _, c := range snap.Cycles {
		if _, err := fmt.Fprintf(w, "Cycle: %s\n", strings.Join(c, " -> ")); err != nil {
			return fmt.Errorf("writing cycle: %w", err)
		}
	}
	if _, err := fmt.Fprintf(w, "\nDiagnostics (%d)\n", len(snap.Diagnostics)); err != nil {
		return fmt.Errorf("writing diagnostics: %w", err)
	}
	for _, d := range snap.Diagnostics {
		if _, err := fmt.Fprintf(w, "[%s] %s %s: %s\n", d.Severity, d.ID, d.Package, d.Message); err != nil {
			return fmt.Errorf("writing diagnostic: %w", err)
		}
	}
	return nil
}
