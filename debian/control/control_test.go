package control

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestString(t *testing.T) {
	Convey("when there is a long description set", t, func() {
		ctrl := Default()
		ctrl.LongDesc = "hey ho\nbanana boat"

		Convey("it should be printed in the string", func() {
			s := ctrl.String()
			So(s, ShouldContainSubstring, "  hey ho")
			So(s, ShouldContainSubstring, "  banana boat")
		})
	})

	Convey("when a control file is rendered to a string", t, func() {
		ctrl := Default()
		lines := strings.Split(ctrl.String(), "\n")
		last := lines[len(lines)-1]
		So(last, ShouldEqual, "")
	})
}

func TestParse(t *testing.T) {
	Convey("given a default control file string", t, func() {
		s := `
Package: test
Version: 0.0.1
Section: misc
Priority: optional
Architecture: all
Essential: no
Installed-Size: 0
Maintainer: Robert McLeod <robert@autogrow.com>
Homepage: http://example.com
Description: This is a description
  hey ho
  banana boat

`

		Convey("when it is parsed", func() {
			ctrl, err := Parse([]byte(s))
			So(err, ShouldBeNil)

			Convey("it should contain the long description", func() {
				So(ctrl.LongDesc, ShouldEqual, "hey ho\nbanana boat")
			})
		})

	})
}

func TestFilename(t *testing.T) {
	Convey("given a control file", t, func() {
		c := Control{Name: "my-app", Version: "1.2.3", Arch: "amd64"}

		Convey("the deb filename follows the dpkg convention", func() {
			So(c.Filename(), ShouldEqual, "my-app_1.2.3_amd64.deb")
		})

		Convey("the default control file gives an all-arch name", func() {
			So(Default("thing").Filename(), ShouldEqual, "thing_0.0.1_all.deb")
		})
	})
}

func TestDefault(t *testing.T) {
	Convey("given no name", t, func() {
		Convey("a placeholder name is used", func() {
			So(Default().Name, ShouldEqual, "my-package")
		})
	})

	Convey("given a name", t, func() {
		Convey("it is used, and the rest are sane defaults", func() {
			c := Default("my-app")
			So(c.Name, ShouldEqual, "my-app")
			So(c.Version, ShouldEqual, "0.0.1")
			So(c.Arch, ShouldEqual, "all")
			So(c.Priority, ShouldEqual, "optional")
			So(c.Section, ShouldEqual, "misc")
		})
	})

	Convey("given several names only the first is used", t, func() {
		So(Default("first", "second").Name, ShouldEqual, "first")
	})
}

func TestWriteAndRead(t *testing.T) {
	Convey("given a control file written to disk", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-control")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		fn := filepath.Join(dir, "control")

		c := Default("my-app")
		c.Version = "2.0.0"
		c.Maintainer = "Jane Doe <jane@example.com>"
		c.Depends = []string{"bash", "curl"}

		So(c.WriteFile(fn), ShouldBeNil)

		Convey("reading it back gives the same fields", func() {
			got, err := Read(fn)
			So(err, ShouldBeNil)
			So(got.Name, ShouldEqual, c.Name)
			So(got.Version, ShouldEqual, c.Version)
			So(got.Maintainer, ShouldEqual, c.Maintainer)
			So(got.Depends, ShouldResemble, c.Depends)
			So(got.LongDesc, ShouldEqual, c.LongDesc)
		})

		Convey("writing again replaces the previous contents", func() {
			c.Version = "3.0.0"
			So(c.WriteFile(fn), ShouldBeNil)

			got, err := Read(fn)
			So(err, ShouldBeNil)
			So(got.Version, ShouldEqual, "3.0.0")

			data, err := ioutil.ReadFile(fn)
			So(err, ShouldBeNil)
			So(string(data), ShouldNotContainSubstring, "2.0.0")
		})

		Convey("reading a file that is not there errors", func() {
			_, err := Read(filepath.Join(dir, "nope"))
			So(err, ShouldNotBeNil)
		})
	})
}

func TestWrite(t *testing.T) {
	Convey("given a control file written to a writer", t, func() {
		var buf bytes.Buffer
		c := Default("my-app")
		So(c.Write(&buf), ShouldBeNil)

		Convey("it writes the same text as String", func() {
			So(buf.String(), ShouldEqual, c.String())
		})
	})
}

func TestSerializeRoundTrip(t *testing.T) {
	Convey("given a dependency list", t, func() {
		Convey("it is rendered comma separated", func() {
			So(serialize([]string{"bash", "curl"}), ShouldEqual, "bash, curl")
			So(serialize([]string{"bash"}), ShouldEqual, "bash")
			So(serialize(nil), ShouldEqual, "")
		})

		Convey("and parsed back with the whitespace trimmed", func() {
			So(unserialize("bash, curl"), ShouldResemble, []string{"bash", "curl"})
			So(unserialize("bash,curl"), ShouldResemble, []string{"bash", "curl"})
			So(unserialize("  bash  ,  curl  "), ShouldResemble, []string{"bash", "curl"})
		})

		// a version constraint carries its own spaces, which must survive
		Convey("a versioned dependency survives the round trip", func() {
			deps := []string{"bash (>= 4.0)", "curl"}
			So(unserialize(serialize(deps)), ShouldResemble, deps)
		})

		// removing a dependency used to leave a blank entry behind, which
		// rendered as a stray comma in the field, eg. "Depends: , curl"
		Convey("blank entries do not become stray commas", func() {
			So(serialize([]string{"", "curl", ""}), ShouldEqual, "curl")
			So(serialize([]string{"curl", "  "}), ShouldEqual, "curl")
			So(serialize([]string{"", ""}), ShouldEqual, "")
			So(serialize([]string{}), ShouldEqual, "")
		})
	})
}

func TestStringOmitsEmptyLists(t *testing.T) {
	Convey("given a control file with no dependencies", t, func() {
		c := Default("my-app")

		Convey("the optional list fields are left out entirely", func() {
			s := c.String()
			So(s, ShouldNotContainSubstring, "Depends:")
			So(s, ShouldNotContainSubstring, "Conflicts:")
			So(s, ShouldNotContainSubstring, "Provides:")
			So(s, ShouldNotContainSubstring, "Replaces:")
		})
	})

	// emptying the list leaves it non-nil, which structs does not call zero,
	// so the field used to be written out as a bare "Depends: " line
	Convey("given a control file whose dependencies have all been removed", t, func() {
		c := Default("my-app")
		c.Depends = []string{"curl"}
		c.Depends = c.Depends[:0]

		Convey("the field is left out rather than written empty", func() {
			s := c.String()
			So(s, ShouldNotContainSubstring, "Depends")
		})
	})

	Convey("given a control file whose dependencies are all blank", t, func() {
		c := Default("my-app")
		c.Depends = []string{"", " "}

		Convey("the field is left out rather than written as commas", func() {
			s := c.String()
			So(s, ShouldNotContainSubstring, "Depends")
		})
	})

	Convey("given a control file with dependencies", t, func() {
		c := Default("my-app")
		c.Depends = []string{"bash", "curl"}
		c.Conflicts = []string{"other-app"}

		Convey("they are listed comma separated", func() {
			s := c.String()
			So(s, ShouldContainSubstring, "Depends: bash, curl")
			So(s, ShouldContainSubstring, "Conflicts: other-app")
		})
	})
}

func TestStringFallsBackToTheShortDescription(t *testing.T) {
	Convey("given a control file with no long description", t, func() {
		c := Default("my-app")
		c.Desc = "a short one"
		c.LongDesc = ""

		Convey("the short one is used as the body, since dpkg wants one", func() {
			s := c.String()
			So(s, ShouldContainSubstring, "Description: a short one")
			So(s, ShouldContainSubstring, "  a short one")
		})

		// String must not mutate the control file it renders
		Convey("and the control file itself is left alone", func() {
			_ = c.String()
			So(c.LongDesc, ShouldEqual, "")
		})
	})
}

func TestStringPutsTheLongDescriptionLast(t *testing.T) {
	Convey("given a control file with a long description", t, func() {
		c := Default("my-app")
		c.LongDesc = "line one\nline two"

		lines := strings.Split(strings.TrimRight(c.String(), "\n"), "\n")

		Convey("the indented body is the last thing in the file", func() {
			So(lines[len(lines)-1], ShouldEqual, "  line two")
			So(lines[len(lines)-2], ShouldEqual, "  line one")
		})
	})
}

func TestParseRoundTrip(t *testing.T) {
	Convey("given a rendered control file", t, func() {
		c := Default("my-app")
		c.Version = "1.2.3"
		c.Maintainer = "Jane Doe <jane@example.com>"
		c.Homepage = "https://example.com/a:b"
		c.Depends = []string{"bash (>= 4.0)", "curl"}
		c.Size = "1024"

		Convey("parsing it back gives the same control file", func() {
			got, err := Parse([]byte(c.String()))
			So(err, ShouldBeNil)
			So(got, ShouldResemble, c)
		})

		// a URL or a version constraint contains colons, and only the first
		// one separates the field from its value
		Convey("a value containing a colon survives", func() {
			got, err := Parse([]byte(c.String()))
			So(err, ShouldBeNil)
			So(got.Homepage, ShouldEqual, "https://example.com/a:b")
		})
	})
}

func TestParseUnknownFieldsAreIgnored(t *testing.T) {
	Convey("given a control file with a field ian does not model", t, func() {
		c, err := Parse([]byte("Package: my-app\nX-Custom-Thing: whatever\n  a description\n"))

		Convey("it parses, keeping what it does understand", func() {
			So(err, ShouldBeNil)
			So(c.Name, ShouldEqual, "my-app")
			So(c.LongDesc, ShouldEqual, "a description")
		})

		// a field left out of the file keeps its default rather than emptying
		Convey("and fields not in the file keep their defaults", func() {
			So(c.Arch, ShouldEqual, "all")
			So(c.Priority, ShouldEqual, "optional")
		})
	})
}
