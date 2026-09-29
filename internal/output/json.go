package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// JSON writes the snapshot as indented JSON followed by a newline.
func JSON(w io.Writer, snap *model.Snapshot) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snap); err != nil {
		return fmt.Errorf("encoding snapshot: %w", err)
	}
	return nil
}
