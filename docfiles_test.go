package ian

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// newPkg makes an initialized package in a temp dir and returns it
func newPkg(t *testing.T) (*Pkg, string) {
	t.Helper()

	dir, err := ioutil.TempDir("/tmp", "go-ian-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	if err := Initialize(dir); err != nil {
		t.Fatal(err)
	}

	p, err := NewPackage(dir)
	if err != nil {
		t.Fatal(err)
	}

	return p, dir
}

func TestDocFilesWrite(t *testing.T) {
	Convey("given an unsorted doc file list", t, func() {
		d := DocFiles{"docs/zebra.md", "README.md"}

		var buf bytes.Buffer
		_, err := d.Write(&buf)
		So(err, ShouldBeNil)

		Convey("it writes one path per line, sorted", func() {
			So(buf.String(), ShouldEqual, "README.md\ndocs/zebra.md\n")
		})

		Convey("and the original list is left in its own order", func() {
			So(d[0], ShouldEqual, "docs/zebra.md")
		})
	})
}

func TestReadDocFilesMissingFile(t *testing.T) {
	Convey("given no docfiles file at all", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-docfiles")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		Convey("the package simply has no doc files", func() {
			d, err := ReadDocFiles(filepath.Join(dir, "docfiles"))
			So(err, ShouldBeNil)
			So(d, ShouldBeEmpty)
		})
	})
}

func TestDocPaths(t *testing.T) {
	Convey("given a package named after its directory", t, func() {
		p, dir := newPkg(t)

		Convey("the doc dir is under usr/share/doc", func() {
			So(p.DocDir(), ShouldEqual, filepath.Join("usr", "share", "doc", filepath.Base(dir)))
		})

		Convey("a doc file's destination is flattened to its base name", func() {
			So(p.DocDest("docs/guide/deep.md"), ShouldEqual, filepath.Join(p.DocDir(), "deep.md"))
			So(p.DocDest("README.md"), ShouldEqual, filepath.Join(p.DocDir(), "README.md"))
		})
	})
}

func TestWriteDocFilesKeepsTheHeader(t *testing.T) {
	Convey("given a freshly initialized package", t, func() {
		p, _ := newPkg(t)

		Convey("its docfiles file starts with the explanatory comment", func() {
			data, err := ioutil.ReadFile(p.DocFilesFile())
			So(err, ShouldBeNil)
			So(string(data), ShouldStartWith, "#")
		})

		Convey("when the list is rewritten", func() {
			So(p.WriteDocFiles(DocFiles{"README.md"}), ShouldBeNil)

			data, err := ioutil.ReadFile(p.DocFilesFile())
			So(err, ShouldBeNil)

			Convey("the comment header survives", func() {
				So(string(data), ShouldStartWith, "#")
				So(string(data), ShouldContainSubstring, "usr/share/doc")
			})

			Convey("and the path is appended after it", func() {
				So(string(data), ShouldEndWith, "README.md\n")
			})

			Convey("so it reads back as just the one path", func() {
				d, err := p.DocFiles()
				So(err, ShouldBeNil)
				So(d, ShouldResemble, DocFiles{"README.md"})
			})
		})

		Convey("rewriting twice does not duplicate the header", func() {
			So(p.WriteDocFiles(DocFiles{"README.md"}), ShouldBeNil)
			So(p.WriteDocFiles(DocFiles{"CHANGELOG.md"}), ShouldBeNil)

			data, err := ioutil.ReadFile(p.DocFilesFile())
			So(err, ShouldBeNil)
			So(strings.Count(string(data), "usr/share/doc"), ShouldEqual, 1)

			Convey("and the old path is gone", func() {
				So(string(data), ShouldNotContainSubstring, "README.md")
			})
		})
	})
}

func TestAddDocFile(t *testing.T) {
	Convey("given an initialized package with a doc file on disk", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, "docs", "guide.md"), "hi\n")

		Convey("when it is registered as a doc file", func() {
			So(p.AddDocFile("docs/guide.md"), ShouldBeNil)

			Convey("it appears in the docfiles list", func() {
				d, err := p.DocFiles()
				So(err, ShouldBeNil)
				So(d, ShouldResemble, DocFiles{"docs/guide.md"})
			})

			// the manifest records where the file ends up in the package, not
			// where it came from, since that is what debsums will check
			Convey("the manifest records its destination and sum", func() {
				m, err := p.Manifest()
				So(err, ShouldBeNil)
				So(len(m), ShouldEqual, 1)
				So(m[0].Path, ShouldEqual, p.DocDest("docs/guide.md"))
				So(m[0].Sum, ShouldEqual, hiSum)
			})

			Convey("the destination maps back to the source", func() {
				srcs, err := p.DocSources()
				So(err, ShouldBeNil)
				So(srcs[p.DocDest("docs/guide.md")], ShouldEqual, "docs/guide.md")
			})

			Convey("registering it again is a no-op rather than a duplicate", func() {
				So(p.AddDocFile("docs/guide.md"), ShouldBeNil)

				d, err := p.DocFiles()
				So(err, ShouldBeNil)
				So(len(d), ShouldEqual, 1)

				m, err := p.Manifest()
				So(err, ShouldBeNil)
				So(len(m), ShouldEqual, 1)
			})

			Convey("and verification covers it through its source file", func() {
				problems, err := p.Verify(false)
				So(err, ShouldBeNil)
				So(problems, ShouldBeEmpty)

				Convey("so changing the source is caught", func() {
					writeFile(t, filepath.Join(dir, "docs", "guide.md"), "changed\n")
					problems, err := p.Verify(false)
					So(err, ShouldNotBeNil)
					So(len(problems), ShouldEqual, 1)

					// the problem names the repo file to go and look at, and
					// the path it installs as
					So(problems[0], ShouldContainSubstring, "docs/guide.md")
					So(problems[0], ShouldContainSubstring, "installs as")
				})
			})
		})

		Convey("a missing file cannot be registered", func() {
			So(p.AddDocFile("docs/nope.md"), ShouldNotBeNil)
		})

		Convey("a path escaping the package is rejected", func() {
			So(p.AddDocFile("../outside.md"), ShouldNotBeNil)
			So(p.AddDocFile("/etc/shadow"), ShouldNotBeNil)
		})

		// a doc file's destination lives in the doc dir, so a file already
		// there would end up being its own source
		Convey("a file already in the doc dir is rejected", func() {
			writeFile(t, filepath.Join(dir, p.DocDir(), "already.md"), "hi\n")
			err := p.AddDocFile(filepath.Join(p.DocDir(), "already.md"))
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "already in the doc directory")
		})

		// destinations are flattened to the base name, so two files with the
		// same name would silently overwrite each other in the package
		Convey("two sources that flatten to the same destination collide", func() {
			writeFile(t, filepath.Join(dir, "other", "guide.md"), "ho\n")
			So(p.AddDocFile("docs/guide.md"), ShouldBeNil)

			err := p.AddDocFile("other/guide.md")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "already installs as")

			Convey("and the colliding file was not registered", func() {
				d, err := p.DocFiles()
				So(err, ShouldBeNil)
				So(len(d), ShouldEqual, 1)
			})
		})
	})
}

func TestRemoveDocFile(t *testing.T) {
	Convey("given a registered doc file", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, "docs", "guide.md"), "hi\n")
		So(p.AddDocFile("docs/guide.md"), ShouldBeNil)

		Convey("when it is removed", func() {
			So(p.RemoveDocFile("docs/guide.md"), ShouldBeNil)

			Convey("it is gone from the docfiles list", func() {
				d, err := p.DocFiles()
				So(err, ShouldBeNil)
				So(d, ShouldBeEmpty)
			})

			Convey("and from the manifest", func() {
				m, err := p.Manifest()
				So(err, ShouldBeNil)
				So(m, ShouldBeEmpty)
			})

			// unregistering only means the file is no longer packaged
			Convey("but the file itself is left on disk", func() {
				So(fexists(filepath.Join(dir, "docs", "guide.md")), ShouldBeTrue)
			})
		})

		Convey("removing one doc file leaves the others alone", func() {
			writeFile(t, filepath.Join(dir, "docs", "other.md"), "ho\n")
			So(p.AddDocFile("docs/other.md"), ShouldBeNil)

			So(p.RemoveDocFile("docs/guide.md"), ShouldBeNil)

			d, err := p.DocFiles()
			So(err, ShouldBeNil)
			So(d, ShouldResemble, DocFiles{"docs/other.md"})

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(len(m), ShouldEqual, 1)
			So(m[0].Path, ShouldEqual, p.DocDest("docs/other.md"))
		})

		Convey("removing one that was never registered errors", func() {
			err := p.RemoveDocFile("docs/nope.md")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "not registered as a doc file")
		})

		Convey("an escaping path is rejected", func() {
			So(p.RemoveDocFile("../outside.md"), ShouldNotBeNil)
		})
	})
}

func TestSourceFor(t *testing.T) {
	Convey("given a package with one doc file and one ordinary file", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, "docs", "guide.md"), "hi\n")
		writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "hi\n")
		So(p.AddDocFile("docs/guide.md"), ShouldBeNil)
		So(p.AddFile("usr/bin/app"), ShouldBeNil)

		srcs, err := p.DocSources()
		So(err, ShouldBeNil)

		Convey("a doc entry resolves back to the file it is copied from", func() {
			So(p.SourceFor(p.DocDest("docs/guide.md"), srcs), ShouldEqual, "docs/guide.md")
		})

		Convey("an ordinary entry is its own source", func() {
			So(p.SourceFor("usr/bin/app", srcs), ShouldEqual, "usr/bin/app")
		})
	})
}

func TestDocSourcesDetectsCollisions(t *testing.T) {
	Convey("given a hand edited docfiles list with two colliding paths", t, func() {
		p, dir := newPkg(t)

		// AddDocFile refuses this, but the list is committed and editable by
		// hand, so the collision has to be caught on read too
		writeFile(t, p.DocFilesFile(), "docs/guide.md\nother/guide.md\n")
		_ = dir

		Convey("reading the sources errors rather than silently dropping one", func() {
			_, err := p.DocSources()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "both install as")
		})
	})
}
