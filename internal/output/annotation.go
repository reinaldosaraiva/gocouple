package output

import (
	"fmt"
	"io"
	"strings"
)

// Annotation is one GitHub Actions workflow command.
type Annotation struct {
	Level   string
	Title   string
	Message string
}

var (
	dataEscaper     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	propertyEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

// WriteAnnotations writes ::error and ::warning workflow commands with the
// escaping GitHub requires.
func WriteAnnotations(w io.Writer, annotations []Annotation) error {
	for _, a := range annotations {
		level := a.Level
		if level != "error" && level != "warning" && level != "notice" {
			level = "warning"
		}
		if _, err := fmt.Fprintf(w, "::%s title=%s::%s\n", level, propertyEscaper.Replace(a.Title), dataEscaper.Replace(a.Message)); err != nil {
			return fmt.Errorf("writing annotation: %w", err)
		}
	}
	return nil
}
