package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// HistoryJSON writes the history as indented JSON followed by a newline.
func HistoryJSON(w io.Writer, h *model.History) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(h); err != nil {
		return fmt.Errorf("encoding history: %w", err)
	}
	return nil
}

// HistoryCSV writes one row per (commit, package), oldest commit first.
// Commits that failed to load produce no rows.
func HistoryCSV(w io.Writer, h *model.History) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"sha", "date", "package", "ca", "ce", "i", "na", "nc", "a", "d", "zone"}); err != nil {
		return fmt.Errorf("writing history csv: %w", err)
	}
	ratio := func(r model.Ratio) string { return strconv.FormatFloat(float64(r), 'f', 4, 64) }
	for _, e := range h.Snapshots {
		if e.Snapshot == nil {
			continue
		}
		for _, p := range e.Snapshot.Packages {
			row := []string{
				e.Commit.SHA, e.Commit.Date, p.Path, strconv.Itoa(p.Ca), strconv.Itoa(p.Ce), ratio(p.Instability),
				strconv.Itoa(p.Na), strconv.Itoa(p.Nc), ratio(p.Abstractness), ratio(p.Distance), p.Zone,
			}
			if err := cw.Write(row); err != nil {
				return fmt.Errorf("writing history csv: %w", err)
			}
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("writing history csv: %w", err)
	}
	return nil
}
