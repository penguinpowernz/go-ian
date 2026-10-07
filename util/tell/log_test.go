package tell

import (
	"bytes"
	"errors"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// capture redirects the log output into a buffer for the duration of fn, and
// returns what was written
func capture(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	fn()

	return buf.String()
}

// atLevel runs fn with the log level set, restoring it afterwards
func atLevel(t *testing.T, level int, fn func()) string {
	t.Helper()

	old := Level
	defer func() { Level = old }()
	Level = level

	return capture(t, fn)
}

func TestLevelPrefixes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fn     func(string, ...interface{})
		prefix string
	}{
		{"debug", Debugf, "DEBUG:"},
		{"info", Infof, "INFO:"},
		{"warn", Warnf, "WARN:"},
		{"error", Errorf, "ERROR:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := atLevel(t, DEBUG, func() { tc.fn("something happened") })

			if !strings.Contains(out, tc.prefix) {
				t.Errorf("output %q does not carry the %s prefix", out, tc.prefix)
			}
			if !strings.Contains(out, "something happened") {
				t.Errorf("output %q does not carry the message", out)
			}
		})
	}
}

func TestFormatting(t *testing.T) {
	out := atLevel(t, DEBUG, func() { Infof("pushed to %d / %d targets", 2, 3) })

	if !strings.Contains(out, "pushed to 2 / 3 targets") {
		t.Errorf("arguments not formatted into the message: %q", out)
	}
}

// a message containing a % must not be re-interpreted as a format string when
// it is logged with no arguments
func TestFormattingWithNoArgs(t *testing.T) {
	out := atLevel(t, DEBUG, func() { Infof("100%% done") })

	if !strings.Contains(out, "100% done") {
		t.Errorf("literal percent mangled: %q", out)
	}
}

func TestLevelFiltering(t *testing.T) {
	for _, tc := range []struct {
		name  string
		level int
		want  map[string]bool // prefix -> should appear
	}{
		{"at debug everything is logged", DEBUG, map[string]bool{
			"DEBUG:": true, "INFO:": true, "WARN:": true, "ERROR:": true,
		}},
		{"at info debug is dropped", INFO, map[string]bool{
			"DEBUG:": false, "INFO:": true, "WARN:": true, "ERROR:": true,
		}},
		{"at warn only warnings and above", WARN, map[string]bool{
			"DEBUG:": false, "INFO:": false, "WARN:": true, "ERROR:": true,
		}},
		{"at error only errors", ERROR, map[string]bool{
			"DEBUG:": false, "INFO:": false, "WARN:": false, "ERROR:": true,
		}},
		{"at fatal nothing below it is logged", FATAL, map[string]bool{
			"DEBUG:": false, "INFO:": false, "WARN:": false, "ERROR:": false,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := atLevel(t, tc.level, func() {
				Debugf("d")
				Infof("i")
				Warnf("w")
				Errorf("e")
			})

			for prefix, want := range tc.want {
				if got := strings.Contains(out, prefix); got != want {
					t.Errorf("%s present = %v, want %v (output %q)", prefix, got, want, out)
				}
			}
		})
	}
}

func TestIfErrorf(t *testing.T) {
	t.Run("an error is logged with its context", func(t *testing.T) {
		out := atLevel(t, DEBUG, func() {
			IfErrorf(errors.New("permission denied"), "running %s failed", "dpkg-deb")
		})

		if !strings.Contains(out, "ERROR:") {
			t.Errorf("not logged as an error: %q", out)
		}
		if !strings.Contains(out, "running dpkg-deb failed") {
			t.Errorf("context missing: %q", out)
		}
		if !strings.Contains(out, "permission denied") {
			t.Errorf("error missing: %q", out)
		}
	})

	// this is called on every push whether or not it failed, so a nil error
	// must be silent
	t.Run("a nil error logs nothing", func(t *testing.T) {
		out := atLevel(t, DEBUG, func() { IfErrorf(nil, "running %s failed", "dpkg-deb") })

		if out != "" {
			t.Errorf("logged %q for a nil error", out)
		}
	})
}

// The fatal paths call os.Exit, so they are checked by re-running this test
// binary as a subprocess and inspecting how it died.
func TestFatalf(t *testing.T) {
	if os.Getenv("TELL_TEST_FATAL") == "1" {
		Fatalf("could not build %s", "the package")
		return
	}

	out, err := runSubprocess(t, "TestFatalf", "TELL_TEST_FATAL=1")

	if err == nil {
		t.Error("Fatalf did not exit non-zero")
	}
	if !strings.Contains(out, "FATAL:") {
		t.Errorf("output does not carry the FATAL prefix: %q", out)
	}
	if !strings.Contains(out, "could not build the package") {
		t.Errorf("output does not carry the message: %q", out)
	}
}

func TestFatalfIgnoresTheLevel(t *testing.T) {
	if os.Getenv("TELL_TEST_FATAL_LEVEL") == "1" {
		// a fatal message is the last thing the process says, so it is
		// logged whatever the level is set to
		Level = FATAL + 1
		Fatalf("the end")
		return
	}

	out, err := runSubprocess(t, "TestFatalfIgnoresTheLevel", "TELL_TEST_FATAL_LEVEL=1")

	if err == nil {
		t.Error("Fatalf did not exit non-zero")
	}
	if !strings.Contains(out, "the end") {
		t.Errorf("fatal message suppressed by the level: %q", out)
	}
}

func TestIfFatalf(t *testing.T) {
	if os.Getenv("TELL_TEST_IFFATAL") == "1" {
		IfFatalf(errors.New("no such file"), "reading %s", "control")
		return
	}

	out, err := runSubprocess(t, "TestIfFatalf", "TELL_TEST_IFFATAL=1")

	if err == nil {
		t.Error("IfFatalf did not exit non-zero")
	}
	if !strings.Contains(out, "reading control: no such file") {
		t.Errorf("output does not carry the context and error: %q", out)
	}
}

func TestIfFatalfNilErrorCarriesOn(t *testing.T) {
	// this guards most of ian's command plumbing, so a nil error must not
	// bring the process down
	out := capture(t, func() { IfFatalf(nil, "reading %s", "control") })

	if out != "" {
		t.Errorf("logged %q for a nil error", out)
	}
}

func TestIfEmptyFatal(t *testing.T) {
	if os.Getenv("TELL_TEST_EMPTY") == "1" {
		IfEmptyFatal("", "the package name")
		return
	}

	out, err := runSubprocess(t, "TestIfEmptyFatal", "TELL_TEST_EMPTY=1")

	if err == nil {
		t.Error("IfEmptyFatal did not exit non-zero for an empty string")
	}
	if !strings.Contains(out, "the package name must not be empty") {
		t.Errorf("output does not name what was empty: %q", out)
	}
}

func TestIfEmptyFatalNonEmptyCarriesOn(t *testing.T) {
	out := capture(t, func() { IfEmptyFatal("my-package", "the package name") })

	if out != "" {
		t.Errorf("logged %q for a non-empty string", out)
	}
}

func TestSetOutput(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	old := Level
	defer func() { Level = old }()
	Level = DEBUG

	Infof("redirected")

	if !strings.Contains(buf.String(), "redirected") {
		t.Errorf("SetOutput did not redirect the log: %q", buf.String())
	}
}

// runSubprocess re-runs this test binary for the named test with the given
// environment set, returning its combined output
func runSubprocess(t *testing.T, name, env string) (string, error) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^"+name+"$")
	cmd.Env = append(os.Environ(), env)

	out, err := cmd.CombinedOutput()
	return string(out), err
}
