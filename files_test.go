package ian

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPkgPaths(t *testing.T) {
	Convey("given a package in a directory", t, func() {
		p := &Pkg{dir: "/tmp/mypkg"}
		p.ctrl.Name = "mypkg"
		p.ctrl.Version = "1.2.3"
		p.ctrl.Arch = "amd64"

		Convey("the control paths sit under DEBIAN", func() {
			So(p.CtrlDir(), ShouldEqual, "/tmp/mypkg/DEBIAN")
			So(p.CtrlFile(), ShouldEqual, "/tmp/mypkg/DEBIAN/control")
			So(p.CtrlDir("postinst"), ShouldEqual, "/tmp/mypkg/DEBIAN/postinst")
			So(p.ManifestFile(), ShouldEqual, "/tmp/mypkg/DEBIAN/md5sums.amd64")
			So(p.DocFilesFile(), ShouldEqual, "/tmp/mypkg/DEBIAN/docfiles")
		})

		Convey("Dir joins onto the package root", func() {
			So(p.Dir(), ShouldEqual, "/tmp/mypkg")
			So(p.Dir("usr", "bin", "app"), ShouldEqual, "/tmp/mypkg/usr/bin/app")
		})

		Convey("the doc path is the absolute form of the doc dir", func() {
			So(p.DocPath(), ShouldEqual, "/tmp/mypkg/usr/share/doc/mypkg")
			So(p.DocPath(), ShouldEqual, p.Dir(p.DocDir()))
		})

		Convey("the deb lands in pkg/ under the control file's name", func() {
			So(p.DebFile(), ShouldEqual, "/tmp/mypkg/pkg/mypkg_1.2.3_amd64.deb")
		})

		Convey("the push file is a dotfile in the package root", func() {
			So(p.PushFile(), ShouldEqual, "/tmp/mypkg/.ianpush")
		})
	})
}

func TestManifestFilePerArch(t *testing.T) {
	Convey("given an initialized package", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-test")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)
		So(Initialize(dir), ShouldBeNil)

		p, err := NewPackage(dir)
		So(err, ShouldBeNil)

		Convey("an arch of all uses the plain manifest", func() {
			p.ctrl.Arch = "all"
			So(p.ManifestFile(), ShouldEqual, filepath.Join(dir, "DEBIAN", "md5sums"))

			Convey("even when an arch-qualified manifest happens to exist", func() {
				So(os.WriteFile(filepath.Join(dir, "DEBIAN", "md5sums.all"), nil, 0644), ShouldBeNil)
				So(p.ManifestFile(), ShouldEqual, filepath.Join(dir, "DEBIAN", "md5sums"))
			})
		})

		Convey("an unset arch uses the plain manifest", func() {
			p.ctrl.Arch = ""
			So(p.ManifestFile(), ShouldEqual, filepath.Join(dir, "DEBIAN", "md5sums"))
		})

		Convey("a concrete arch falls back to the plain manifest while it is the only one", func() {
			p.ctrl.Arch = "amd64"
			So(p.ManifestFile(), ShouldEqual, filepath.Join(dir, "DEBIAN", "md5sums"))

			Convey("and prefers the arch-qualified manifest once it exists", func() {
				So(os.WriteFile(filepath.Join(dir, "DEBIAN", "md5sums.amd64"), nil, 0644), ShouldBeNil)
				So(p.ManifestFile(), ShouldEqual, filepath.Join(dir, "DEBIAN", "md5sums.amd64"))

				Convey("while another arch still falls back", func() {
					p.ctrl.Arch = "arm64"
					So(p.ManifestFile(), ShouldEqual, filepath.Join(dir, "DEBIAN", "md5sums"))
				})
			})
		})

		Convey("with no plain manifest at all a concrete arch names its own", func() {
			So(os.Remove(filepath.Join(dir, "DEBIAN", "md5sums")), ShouldBeNil)
			p.ctrl.Arch = "armhf"
			So(p.ManifestFile(), ShouldEqual, filepath.Join(dir, "DEBIAN", "md5sums.armhf"))
		})

		Convey("ManifestFiles lists the plain manifest and every sibling", func() {
			for _, n := range []string{"md5sums.amd64", "md5sums.arm64"} {
				So(os.WriteFile(filepath.Join(dir, "DEBIAN", n), nil, 0644), ShouldBeNil)
			}

			So(p.ManifestFiles(), ShouldResemble, []string{
				filepath.Join(dir, "DEBIAN", "md5sums"),
				filepath.Join(dir, "DEBIAN", "md5sums.amd64"),
				filepath.Join(dir, "DEBIAN", "md5sums.arm64"),
			})
		})
	})
}

func TestIsManifestFile(t *testing.T) {
	Convey("manifests are recognized by name", t, func() {
		So(IsManifestFile("md5sums"), ShouldBeTrue)
		So(IsManifestFile("md5sums.amd64"), ShouldBeTrue)
		So(IsManifestFile("md5sums.all"), ShouldBeTrue)

		Convey("and other control files are not", func() {
			So(IsManifestFile("control"), ShouldBeFalse)
			So(IsManifestFile("docfiles"), ShouldBeFalse)
			So(IsManifestFile("postinst"), ShouldBeFalse)
			So(IsManifestFile("md5sum"), ShouldBeFalse)
			So(IsManifestFile("conffiles"), ShouldBeFalse)
		})
	})
}

func TestInitializedAndIsInitialized(t *testing.T) {
	Convey("given a bare directory", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-test")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		Convey("it is not initialized", func() {
			So(IsInitialized(dir), ShouldBeFalse)
			So((&Pkg{dir: dir}).Initialized(), ShouldBeFalse)
		})

		Convey("once initialized it reports as such", func() {
			So(Initialize(dir), ShouldBeNil)
			So(IsInitialized(dir), ShouldBeTrue)

			p, err := NewPackage(dir)
			So(err, ShouldBeNil)
			So(p.Initialized(), ShouldBeTrue)

			Convey("and initializing again errors rather than clobbering it", func() {
				err := Initialize(dir)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "already initialized")
			})
		})

		// a DEBIAN dir with no control file in it is not a package
		Convey("a control dir without a control file is not initialized", func() {
			So(os.MkdirAll(filepath.Join(dir, "DEBIAN"), 0755), ShouldBeNil)
			So(IsInitialized(dir), ShouldBeFalse)
		})
	})
}

func TestNewPackageReadsTheControlFile(t *testing.T) {
	Convey("given an initialized package", t, func() {
		p, dir := newPkg(t)

		Convey("the control file is read in", func() {
			So(p.Ctrl().Name, ShouldEqual, filepath.Base(dir))
			So(p.Ctrl().Version, ShouldEqual, "0.0.1")
		})

		Convey("and the returned control object is editable in place", func() {
			p.Ctrl().Version = "9.9.9"
			So(p.Ctrl().Version, ShouldEqual, "9.9.9")
			So(p.DebFile(), ShouldContainSubstring, "9.9.9")
		})
	})

	Convey("given a directory that is not a package", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-test")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		Convey("reading it errors", func() {
			_, err := NewPackage(dir)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestCtrlFiles(t *testing.T) {
	Convey("given an initialized package", t, func() {
		p, _ := newPkg(t)

		Convey("the control files are listed", func() {
			names := map[string]bool{}
			for _, f := range p.CtrlFiles() {
				names[filepath.Base(f)] = true
			}

			for _, want := range []string{"control", "md5sums", "docfiles", "postinst", "prerm", "postrm", "preinst"} {
				So(names[want], ShouldBeTrue)
			}
		})
	})
}

func TestSize(t *testing.T) {
	Convey("given a package with registered files", t, func() {
		p, dir := newPkg(t)

		Convey("with nothing registered the size is zero", func() {
			size, err := p.Size()
			So(err, ShouldBeNil)
			So(size, ShouldEqual, "0")
		})

		Convey("the size is summed from the registered files, in kB", func() {
			writeFile(t, filepath.Join(dir, "usr", "bin", "app"), string(make([]byte, 2048)))
			So(p.AddFile("usr/bin/app"), ShouldBeNil)

			size, err := p.Size()
			So(err, ShouldBeNil)
			So(size, ShouldEqual, "2")
		})

		// a doc file is summed from the file in the repo, since the
		// destination path does not exist until the package is staged
		Convey("a doc file is sized from its source in the repo", func() {
			writeFile(t, filepath.Join(dir, "docs", "guide.md"), string(make([]byte, 1024)))
			So(p.AddDocFile("docs/guide.md"), ShouldBeNil)

			size, err := p.Size()
			So(err, ShouldBeNil)
			So(size, ShouldEqual, "1")
		})

		Convey("a registered file that has gone missing errors", func() {
			writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "hi\n")
			So(p.AddFile("usr/bin/app"), ShouldBeNil)
			So(os.Remove(filepath.Join(dir, "usr", "bin", "app")), ShouldBeNil)

			_, err := p.Size()
			So(err, ShouldNotBeNil)
		})
	})
}
