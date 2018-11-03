package ian

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

var targetList = []byte(`true do nothing
true do nothingness
stable: true do nothing named
test: true do nothingness named
`)

func TestParseTargets(t *testing.T) {
	Convey("given a package name", t, func() {
		pkg := "pkg/test.deb"

		Convey("when parsing a target list", func() {
			tgts := parseTargets(targetList, pkg)
			So(tgts, ShouldHaveLength, 4)

			Convey("it should have 2 default targets", func() {
				var c int
				for _, tg := range tgts {
					if tg.name == "default" {
						c++
					}
				}
				So(c, ShouldEqual, 2)
			})

			Convey("default targets should not be named", func() {
				for _, tg := range tgts {
					if tg.name == "default" {
						So(tg.cmd.Args[len(tg.cmd.Args)-1], ShouldNotEqual, "named")
					}
				}
			})

			Convey("it should have two named targets", func() {
				var c int
				for _, tg := range tgts {
					if tg.name != "default" {
						c++
					}
				}
				So(c, ShouldEqual, 2)
			})

			Convey("named targets should have names", func() {
				var n []string
				for _, tg := range tgts {
					if tg.name != "default" {
						n = append(n, tg.name)
					}
				}
				So(n, ShouldResemble, []string{"stable", "test"})
			})

			Convey("named targets cmds should be named", func() {
				for _, tg := range tgts {
					if tg.name != "default" {
						So(tg.cmd.Args[len(tg.cmd.Args)-2], ShouldEqual, "named")
					}
				}
			})
		})
	})
}

func TestSelectTargets(t *testing.T) {
	Convey("given some targets", t, func() {
		tgts := []*target{
			&target{name: "default"},
			&target{name: "default"},
			&target{name: "stable"},
			&target{name: "test"},
			&target{name: "staging"},
		}

		Convey("it should select the right targets", func() {
			ts := selectTargets(tgts, "")
			So(len(ts), ShouldEqual, 0)

			ts = selectTargets(tgts, "default")
			So(len(ts), ShouldEqual, 2)
			So(ts[0].name, ShouldEqual, "default")
			So(ts[1].name, ShouldEqual, "default")

			ts = selectTargets(tgts, "sta*")
			So(len(ts), ShouldEqual, 2)
			So(ts[0].name, ShouldEqual, "stable")
			So(ts[1].name, ShouldEqual, "staging")

			ts = selectTargets(tgts, "stable")
			So(len(ts), ShouldEqual, 1)
			So(ts[0].name, ShouldEqual, "stable")

			ts = selectTargets(tgts, "test")
			So(len(ts), ShouldEqual, 1)
			So(ts[0].name, ShouldEqual, "test")

			ts = selectTargets(tgts, "*ult")
			So(len(ts), ShouldEqual, 2)
			So(ts[0].name, ShouldEqual, "default")
			So(ts[1].name, ShouldEqual, "default")
		})
	})
}

func TestPushMakeCmd(t *testing.T) {
	Convey("given a package name", t, func() {
		pkg := "pkg/test.deb"
		Convey("and a command string", func() {
			Convey("without pkg placeholder", func() {
				s := "/bin/true do nothing"

				Convey("when making the package", func() {
					Convey("the package name should appear on the end", func() {
						cmd, err := makeCmd(s, pkg)
						So(err, ShouldBeNil)
						So(cmd.Args[len(cmd.Args)-1], ShouldEqual, pkg)
					})
				})
			})

			Convey("with pkg placeholder", func() {
				s := "/bin/true do $PKG nothing"
				Convey("when making the package", func() {
					Convey("the package name should appear inline", func() {
						cmd, err := makeCmd(s, pkg)
						So(err, ShouldBeNil)
						So(cmd.Args[2], ShouldEqual, pkg)
					})
				})
			})

			Convey("without absolute executable path", func() {
				s := "true do nothing"
				cmd, err := makeCmd(s, pkg)
				So(err, ShouldBeNil)
				Convey("it should find absolute executable path", func() {
					So(cmd.Args[0], ShouldEqual, "/bin/true")
				})
			})
		})
	})
}
