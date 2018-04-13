package control

import (
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
