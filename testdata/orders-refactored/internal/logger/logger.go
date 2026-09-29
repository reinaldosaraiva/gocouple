package logger

import (
	"fmt"
	"os"

	"example.com/ordersrefactored/internal/contracts"
)

type Stderr struct {
	fields contracts.Fields
}

func New() *Stderr { return &Stderr{} }

func (l *Stderr) Printf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func (l *Stderr) With(fields contracts.Fields) contracts.Logger {
	return &Stderr{fields: fields}
}
