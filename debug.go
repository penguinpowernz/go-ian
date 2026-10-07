package ian

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// debugWidth is the width of the rules drawn around a debug section
const debugWidth = 72

// the SGR escapes used for the headings.  Status words are left uncoloured
// so that the file listings stay greppable and quiet.
const (
	ansiReset = "\033[0m"
	ansiStep  = "\033[1;36m" // bold cyan, for the step heading
	ansiRule  = "\033[2;37m" // dim grey, for the rules
	ansiSect  = "\033[1m"    // bold, for a section title
	ansiSum   = "\033[2m"    // dim, for the step summary
)

// debug is the debug output printer for a build.  All of the packaging steps
// report through it so that a debug build reads as one consistent document:
// a heading per step, indented sections under it, and aligned status lines
// within each section.  When off, every call is a no-op.
type debug struct {
	w  io.Writer
	on bool

	// colour is set when the output is a terminal, so that a redirected or
	// piped debug log stays free of escape sequences
	colour bool

	// open is the section currently being written to, if any
	open bool
}

// newDebug returns a debug printer writing to stderr
func newDebug(on bool) *debug {
	return &debug{w: os.Stderr, on: on, colour: isTerminal(os.Stderr)}
}

// isTerminal reports whether the file is a character device, which is the
// closest the standard library gets to a TTY check.  NO_COLOR is honoured
// so the headings can be turned plain without turning off debug output.
func isTerminal(f *os.File) bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}

	fi, err := f.Stat()
	if err != nil {
		return false
	}

	return fi.Mode()&os.ModeCharDevice != 0
}

// paint wraps s in an SGR escape, or returns it unchanged when the output
// is not a terminal
func (d *debug) paint(code, s string) string {
	if !d.colour {
		return s
	}

	return code + s + ansiReset
}

// Step announces a packaging step, which is the top level of the output
func (d *debug) Step(name string) {
	if d == nil || !d.on {
		return
	}

	d.EndSection()
	fmt.Fprintf(d.w, "\n%s\n%s\n",
		d.paint(ansiStep, strings.ToUpper(name)),
		d.paint(ansiRule, strings.Repeat("=", debugWidth)))
}

// Section begins a titled block of lines within the current step.  Lines
// written until the next Section, EndSection or Step belong to it.
func (d *debug) Section(format string, args ...interface{}) {
	if d == nil || !d.on {
		return
	}

	d.EndSection()
	fmt.Fprintf(d.w, "\n%s\n%s\n",
		d.paint(ansiSect, fmt.Sprintf(format, args...)),
		d.paint(ansiRule, strings.Repeat("-", debugWidth)))
	d.open = true
}

// EndSection closes the current section, drawing its bottom rule
func (d *debug) EndSection() {
	if d == nil || !d.on || !d.open {
		return
	}

	fmt.Fprintf(d.w, "%s\n", d.paint(ansiRule, strings.Repeat("-", debugWidth)))
	d.open = false
}

// Printf writes a plain line, indented when inside a section
func (d *debug) Printf(format string, args ...interface{}) {
	if d == nil || !d.on {
		return
	}

	indent := ""
	if d.open {
		indent = "  "
	}

	fmt.Fprintf(d.w, "%s%s\n", indent, fmt.Sprintf(format, args...))
}

// Write copies raw bytes through verbatim, for including file contents
func (d *debug) Write(b []byte) {
	if d == nil || !d.on {
		return
	}

	_, _ = d.w.Write(b)
	if len(b) > 0 && b[len(b)-1] != '\n' {
		fmt.Fprintln(d.w)
	}
}

// File reports the state of one file as an aligned status line: a status
// word, the sum it was checked against, the path, and any detail.  Status
// words are upper case when something is wrong so they stand out from the
// lower case "ok" of a file that checked out.
func (d *debug) File(status, sum, path, detail string, args ...interface{}) {
	if d == nil || !d.on {
		return
	}

	// pad before painting: the escape bytes would otherwise be counted in
	// the field width and throw the columns out
	word := d.paint(statusColour(status), fmt.Sprintf("%-9s", status))

	// a line with no sum to show (a staged file, an unregistered one) closes
	// the gap rather than leaving an empty column
	line := fmt.Sprintf("  %s %s", word, path)
	if sum != "" {
		line = fmt.Sprintf("  %s %-32s %s", word, sum, path)
	}

	if detail != "" {
		line += " (" + fmt.Sprintf(detail, args...) + ")"
	}

	fmt.Fprintln(d.w, line)
}

// statusColour returns the escape for a status word: green when the file
// checked out, yellow when it is merely unexpected, red when it is wrong
func statusColour(status string) string {
	switch status {
	case "ok", "staged", "control":
		return "\033[32m"
	case "UNKNOWN", "ABSENT", "SKIPPED":
		return "\033[33m"
	default:
		return "\033[1;31m"
	}
}

// Summary closes any open section and reports the result of a step
func (d *debug) Summary(format string, args ...interface{}) {
	if d == nil || !d.on {
		return
	}

	d.EndSection()
	fmt.Fprintf(d.w, "\n%s\n", d.paint(ansiSum, "=> "+fmt.Sprintf(format, args...)))
}
