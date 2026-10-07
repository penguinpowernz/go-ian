package ian

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func fexists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func TestInit(t *testing.T) {
	Convey("given a directory", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-test")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		Convey("when it is initialized", func() {
			So(fexists(dir), ShouldBeTrue)
			err = Initialize(dir)
			So(err, ShouldBeNil)

			Convey("it should have the required files in it", func() {
				So(fexists(dir+"/DEBIAN/postinst"), ShouldBeTrue)
				So(fexists(dir+"/DEBIAN/md5sums"), ShouldBeTrue)
				So(fexists(dir+"/DEBIAN/control"), ShouldBeTrue)
			})
		})
	})
}

func TestInitializeContents(t *testing.T) {
	Convey("given a freshly initialized package", t, func() {
		p, dir := newPkg(t)

		Convey("the package is named after its directory", func() {
			So(p.Ctrl().Name, ShouldEqual, filepath.Base(dir))
		})

		Convey("all four maintainer scripts are stubbed out", func() {
			for _, name := range []string{"preinst", "postinst", "prerm", "postrm"} {
				So(fexists(p.CtrlDir(name)), ShouldBeTrue)
			}
		})

		// nothing is in the package until the developer registers it
		Convey("the manifest starts empty", func() {
			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m, ShouldBeEmpty)

			fi, err := os.Stat(p.ManifestFile())
			So(err, ShouldBeNil)
			So(fi.Size(), ShouldEqual, 0)
		})

		Convey("the docfiles list starts with just its explanation", func() {
			d, err := p.DocFiles()
			So(err, ShouldBeNil)
			So(d, ShouldBeEmpty)

			data, err := ioutil.ReadFile(p.DocFilesFile())
			So(err, ShouldBeNil)
			So(string(data), ShouldEqual, defaultDocFiles)
		})

		Convey("the push file is created empty", func() {
			So(fexists(p.PushFile()), ShouldBeTrue)
		})
	})
}

func TestFindMaintainer(t *testing.T) {
	Convey("given a HOME with a gitconfig", t, func() {
		home, err := ioutil.TempDir("/tmp", "go-ian-home")
		So(err, ShouldBeNil)
		defer os.RemoveAll(home)

		gc := filepath.Join(home, ".gitconfig")

		Convey("a name and email become the maintainer line", func() {
			writeFile(t, gc, "[user]\n\tname = Jane Doe\n\temail = jane@example.com\n")
			t.Setenv("HOME", home)

			mntr, ok := FindMaintainer()
			So(ok, ShouldBeTrue)
			So(mntr, ShouldEqual, "Jane Doe <jane@example.com>")
		})

		// a half filled gitconfig would otherwise give a malformed
		// maintainer line, which dpkg rejects
		Convey("a gitconfig missing either field is no use", func() {
			for _, contents := range []string{
				"[user]\n\tname = Jane Doe\n",
				"[user]\n\temail = jane@example.com\n",
				"[core]\n\teditor = vim\n",
			} {
				writeFile(t, gc, contents)
				t.Setenv("HOME", home)

				_, ok := FindMaintainer()
				So(ok, ShouldBeFalse)
			}
		})

		Convey("an email containing an = is kept whole", func() {
			writeFile(t, gc, "[user]\n\tname = Jane\n\temail = jane+a=b@example.com\n")
			t.Setenv("HOME", home)

			mntr, ok := FindMaintainer()
			So(ok, ShouldBeTrue)
			So(mntr, ShouldEqual, "Jane <jane+a=b@example.com>")
		})

		Convey("no gitconfig at all means no maintainer", func() {
			t.Setenv("HOME", filepath.Join(home, "nowhere"))

			_, ok := FindMaintainer()
			So(ok, ShouldBeFalse)
		})
	})
}

func TestInitializeUsesTheMaintainer(t *testing.T) {
	Convey("given a HOME with a usable gitconfig", t, func() {
		home, err := ioutil.TempDir("/tmp", "go-ian-home")
		So(err, ShouldBeNil)
		defer os.RemoveAll(home)

		writeFile(t, filepath.Join(home, ".gitconfig"), "[user]\n\tname = Jane Doe\n\temail = jane@example.com\n")
		t.Setenv("HOME", home)

		dir, err := ioutil.TempDir("/tmp", "go-ian-test")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		Convey("a new package picks it up as the maintainer", func() {
			So(Initialize(dir), ShouldBeNil)

			p, err := NewPackage(dir)
			So(err, ShouldBeNil)
			So(p.Ctrl().Maintainer, ShouldEqual, "Jane Doe <jane@example.com>")
		})
	})
}
