// Package colour applies ANSI colour to terminal output, and gets out of the
// way when the output is not a terminal so that redirected or piped output
// stays free of escape sequences.
package colour

import (
	"os"
)

// The SGR escapes used across ian's output
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	Red    = "\033[1;31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[1;36m"
	Grey   = "\033[2;37m"
)

// IsTerminal reports whether the file is a character device, which is the
// closest the standard library gets to a TTY check.  NO_COLOR is honoured so
// that colour can be turned off without changing anything else.
func IsTerminal(f *os.File) bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}

	fi, err := f.Stat()
	if err != nil {
		return false
	}

	return fi.Mode()&os.ModeCharDevice != 0
}

// Painter paints strings in a given colour, or returns them unchanged when
// colour is off.  The zero value is a painter with colour off.
type Painter struct {
	On bool
}

// For returns a Painter that paints only if the given file is a terminal
func For(f *os.File) Painter {
	return Painter{On: IsTerminal(f)}
}

// P wraps s in an SGR escape, or returns it unchanged when colour is off
func (p Painter) P(code, s string) string {
	if !p.On || s == "" {
		return s
	}

	return code + s + Reset
}

// Pad left-aligns s in a field of n characters and then paints it.  Padding
// before painting matters: the escape bytes are not printable but would be
// counted by a width directive or by text/tabwriter, throwing columns out.
func (p Painter) Pad(code, s string, n int) string {
	for len(s) < n {
		s += " "
	}

	return p.P(code, s)
}
