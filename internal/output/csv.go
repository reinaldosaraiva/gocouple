package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// CSV writes one row per package sorted by path with ratios at four decimals.
func CSV(w io.Writer, snap *model.Snapshot) error {
	pkgs := slices.Clone(snap.Packages)
	slices.SortFunc(pkgs, func(a, b model.Package) int { return strings.Compare(a.Path, b.Path) })

	cw := csv.NewWriter(w)
	churn := snap.Config.VolatilitySince != ""
	header := []string{"package", "ca", "ce", "i", "na", "nc", "a", "d", "zone"}
	if churn {
		header = append(header, "churn")
	}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("writing csv: %w", err)
	}
	ratio := func(r model.Ratio) string { return strconv.FormatFloat(float64(r), 'f', 4, 64) }
	for _, p := range pkgs {
		row := []string{
			p.Path, strconv.Itoa(p.Ca), strconv.Itoa(p.Ce), ratio(p.Instability),
			strconv.Itoa(p.Na), strconv.Itoa(p.Nc), ratio(p.Abstractness), ratio(p.Distance), p.Zone,
		}
		if churn {
			row = append(row, strconv.Itoa(p.Churn))
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("writing csv: %w", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("writing csv: %w", err)
	}
	return nil
}
