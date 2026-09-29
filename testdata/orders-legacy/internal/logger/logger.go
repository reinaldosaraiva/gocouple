package logger

import (
	"fmt"
	"os"
)

type Logger struct {
	prefix string
}

func New(prefix string) *Logger { return &Logger{prefix: prefix} }

func (l *Logger) Printf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, l.prefix+": "+format+"\n", args...)
}
