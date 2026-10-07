package ian

import (
	"io/ioutil"
	"os"
	"path/filepath"
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

func TestParseTargetsEdgeCases(t *testing.T) {
	Convey("given a push file", t, func() {
		pkg := "pkg/test.deb"

		Convey("an empty one yields no targets", func() {
			So(parseTargets([]byte(""), pkg), ShouldBeEmpty)
			So(parseTargets([]byte("   \n\n  \n"), pkg), ShouldBeEmpty)
		})

		Convey("blank lines between commands are skipped", func() {
			tgts := parseTargets([]byte("true one\n\n\ntrue two\n"), pkg)
			So(len(tgts), ShouldEqual, 2)
		})

		// a line naming a binary that is not installed cannot be run, so it
		// is reported and skipped rather than aborting the whole push
		Convey("a line naming a missing binary is skipped", func() {
			tgts := parseTargets([]byte("definitely-not-a-real-binary-xyzzy go\ntrue ok\n"), pkg)
			So(len(tgts), ShouldEqual, 1)
			So(tgts[0].cmd.Args, ShouldContain, "ok")
		})

		Convey("a named line naming a missing binary is skipped too", func() {
			tgts := parseTargets([]byte("stable: definitely-not-a-real-binary-xyzzy go\n"), pkg)
			So(tgts, ShouldBeEmpty)
		})
	})
}

func TestMakeCmdMissingBinary(t *testing.T) {
	Convey("given a command naming a binary that is not installed", t, func() {
		_, err := makeCmd("definitely-not-a-real-binary-xyzzy go", "pkg/test.deb")

		Convey("it errors, naming the binary", func() {
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "definitely-not-a-real-binary-xyzzy")
		})
	})
}

func TestPush(t *testing.T) {
	Convey("given a package file and a push file", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-push")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		deb := filepath.Join(dir, "test.deb")
		writeFile(t, deb, "not really a deb\n")

		pushFile := filepath.Join(dir, ".ianpush")

		Convey("a push whose command succeeds reports success", func() {
			writeFile(t, pushFile, "true\n")
			So(Push(pushFile, deb, ""), ShouldBeNil)
		})

		Convey("a push whose command fails errors", func() {
			writeFile(t, pushFile, "false\n")
			err := Push(pushFile, deb, "")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "failed to execute")
		})

		// one failing target must not be reported as a clean push
		Convey("a push where only some targets succeed errors", func() {
			writeFile(t, pushFile, "true\nfalse\n")
			So(Push(pushFile, deb, ""), ShouldNotBeNil)
		})

		Convey("named targets are selected by name", func() {
			writeFile(t, pushFile, "stable: true\ntest: false\n")

			So(Push(pushFile, deb, "stable"), ShouldBeNil)
			So(Push(pushFile, deb, "test"), ShouldNotBeNil)
		})

		Convey("a glob selects several targets", func() {
			writeFile(t, pushFile, "stable: true\nstaging: true\ntest: false\n")
			So(Push(pushFile, deb, "sta*"), ShouldBeNil)
		})

		// an empty selector means the unnamed lines, so `ian push` with no
		// argument pushes what the push file lists plainly
		Convey("no selector means the default targets", func() {
			writeFile(t, pushFile, "true\nstable: false\n")
			So(Push(pushFile, deb, ""), ShouldBeNil)
		})

		Convey("a selector matching nothing errors", func() {
			writeFile(t, pushFile, "stable: true\n")
			err := Push(pushFile, deb, "nope")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "no targets")
		})

		Convey("an empty push file errors rather than claiming success", func() {
			writeFile(t, pushFile, "\n")
			So(Push(pushFile, deb, ""), ShouldNotBeNil)
		})

		Convey("a missing push file errors", func() {
			err := Push(filepath.Join(dir, "nope"), deb, "")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "couldn't read push file")
		})

		// pushing a package that was never built is a mistake worth catching
		// before any command runs
		Convey("a missing package errors", func() {
			writeFile(t, pushFile, "true\n")
			err := Push(pushFile, filepath.Join(dir, "nope.deb"), "")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "couldn't find package")
		})
	})
}
