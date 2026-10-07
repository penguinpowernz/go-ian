package ian

import (
	"bytes"
	"strings"
	"testing"

	"github.com/penguinpowernz/go-ian/util/colour"

	. "github.com/smartystreets/goconvey/convey"
)

// newTestDebug returns a debug printer writing into a buffer with colour off,
// so the assertions can be made against plain text
func newTestDebug(on bool) (*debug, *bytes.Buffer) {
	var buf bytes.Buffer
	return &debug{w: &buf, on: on, c: colour.Painter{On: false}}, &buf
}

func TestDebugOff(t *testing.T) {
	Convey("given a debug printer that is off", t, func() {
		d, buf := newTestDebug(false)

		Convey("every call is a no-op", func() {
			d.Step("staging")
			d.Section("files in %s", "/tmp")
			d.Printf("something")
			d.Write([]byte("raw"))
			d.File("ok", "abc", "usr/bin/app", "")
			d.Summary("done")
			d.EndSection()

			So(buf.String(), ShouldEqual, "")
		})
	})

	// the build request's printer is nil when BuildWithOpts was bypassed, so
	// every method has to tolerate it rather than panic mid-build
	Convey("a nil debug printer is safe to call", t, func() {
		var d *debug
		So(func() {
			d.Step("staging")
			d.Section("x")
			d.Printf("x")
			d.Write([]byte("x"))
			d.File("ok", "abc", "x", "")
			d.Summary("x")
			d.EndSection()
		}, ShouldNotPanic)
	})
}

func TestDebugSections(t *testing.T) {
	Convey("given a debug printer that is on", t, func() {
		d, buf := newTestDebug(true)

		Convey("a step is printed in upper case under a rule", func() {
			d.Step("staging the files")
			So(buf.String(), ShouldContainSubstring, "STAGING THE FILES")
			So(buf.String(), ShouldContainSubstring, strings.Repeat("=", debugWidth))
		})

		Convey("a section title is formatted and ruled", func() {
			d.Section("files staged to %s", "/tmp/x")
			So(buf.String(), ShouldContainSubstring, "files staged to /tmp/x")
			So(buf.String(), ShouldContainSubstring, strings.Repeat("-", debugWidth))
		})

		Convey("lines inside a section are indented", func() {
			d.Section("a section")
			buf.Reset()
			d.Printf("a line")
			So(buf.String(), ShouldEqual, "  a line\n")
		})

		Convey("lines outside a section are not indented", func() {
			d.Printf("a line")
			So(buf.String(), ShouldEqual, "a line\n")
		})

		Convey("a section is closed by the next one", func() {
			d.Section("first")
			d.Section("second")

			// the first section's bottom rule, then the second's title
			So(strings.Count(buf.String(), strings.Repeat("-", debugWidth)), ShouldEqual, 3)
		})

		Convey("closing a section twice only draws one bottom rule", func() {
			d.Section("one")
			buf.Reset()
			d.EndSection()
			d.EndSection()
			So(strings.Count(buf.String(), strings.Repeat("-", debugWidth)), ShouldEqual, 1)
		})

		Convey("a summary closes the open section and reports the result", func() {
			d.Section("one")
			buf.Reset()
			d.Summary("staged %d file(s)", 3)
			So(buf.String(), ShouldContainSubstring, strings.Repeat("-", debugWidth))
			So(buf.String(), ShouldContainSubstring, "=> staged 3 file(s)")
		})
	})
}

func TestDebugWrite(t *testing.T) {
	Convey("given a debug printer that is on", t, func() {
		d, buf := newTestDebug(true)

		Convey("raw bytes pass through verbatim", func() {
			d.Write([]byte("Package: app\n"))
			So(buf.String(), ShouldEqual, "Package: app\n")
		})

		// a control file with no trailing newline would otherwise run into
		// the next line of output
		Convey("a missing trailing newline is added", func() {
			d.Write([]byte("Package: app"))
			So(buf.String(), ShouldEqual, "Package: app\n")
		})

		Convey("nothing is written for no bytes", func() {
			d.Write(nil)
			So(buf.String(), ShouldEqual, "")
		})
	})
}

func TestDebugFile(t *testing.T) {
	Convey("given a debug printer that is on", t, func() {
		d, buf := newTestDebug(true)

		Convey("a status line holds the status, sum and path", func() {
			d.File("ok", hiSum, "usr/bin/app", "")
			line := buf.String()
			So(line, ShouldContainSubstring, "ok")
			So(line, ShouldContainSubstring, hiSum)
			So(line, ShouldContainSubstring, "usr/bin/app")
		})

		Convey("the status word is padded so the columns line up", func() {
			d.File("ok", hiSum, "a", "")
			d.File("MISMATCH", hiSum, "b", "")

			// split without trimming: the leading indent is part of the
			// alignment being checked
			lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
			So(len(lines), ShouldEqual, 2)
			So(strings.Index(lines[0], hiSum), ShouldEqual, strings.Index(lines[1], hiSum))
		})

		// a line with no sum closes the gap rather than leaving an empty column
		Convey("a line with no sum has no sum column", func() {
			d.File("staged", "", "usr/bin/app", "")
			So(buf.String(), ShouldEqual, "  staged    usr/bin/app\n")
		})

		Convey("a detail is appended in brackets and formatted", func() {
			d.File("MISMATCH", hiSum, "usr/bin/app", "got %s", "abc")
			So(buf.String(), ShouldContainSubstring, "(got abc)")
		})
	})
}

func TestStatusColour(t *testing.T) {
	Convey("given a status word", t, func() {
		Convey("a file that checked out is green", func() {
			for _, s := range []string{"ok", "staged", "control"} {
				So(statusColour(s), ShouldEqual, colour.Green)
			}
		})

		Convey("a merely unexpected file is yellow", func() {
			for _, s := range []string{"UNKNOWN", "ABSENT", "SKIPPED"} {
				So(statusColour(s), ShouldEqual, colour.Yellow)
			}
		})

		Convey("anything wrong is red", func() {
			for _, s := range []string{"MISSING", "MISMATCH", "ERROR", "DIFFERS"} {
				So(statusColour(s), ShouldEqual, colour.Red)
			}
		})
	})
}

func TestNewDebug(t *testing.T) {
	Convey("a new debug printer carries the on flag", t, func() {
		So(newDebug(true).on, ShouldBeTrue)
		So(newDebug(false).on, ShouldBeFalse)
	})
}
