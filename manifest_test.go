package ian

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// md5 of "hi\n" is 764efa883dda1e11db47671c4a3bbd9e
const hiSum = "764efa883dda1e11db47671c4a3bbd9e"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestReadManifest(t *testing.T) {
	Convey("given an md5sums file", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-manifest")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		Convey("with valid lines and a blank line", func() {
			mf := filepath.Join(dir, "md5sums")
			writeFile(t, mf, "abc  bin/thing\n\ndef  etc/conf\n")

			m, err := ReadManifest(mf)
			So(err, ShouldBeNil)
			So(len(m), ShouldEqual, 2)
			So(m[0].Sum, ShouldEqual, "abc")
			So(m[0].Path, ShouldEqual, "bin/thing")
			So(m[1].Path, ShouldEqual, "etc/conf")
		})

		Convey("with a malformed line it errors", func() {
			mf := filepath.Join(dir, "md5sums")
			writeFile(t, mf, "this line has too many fields here\n")

			_, err := ReadManifest(mf)
			So(err, ShouldNotBeNil)
		})

		// the manifest is committed and hand editable, so a path that escapes
		// the package dir must be rejected on read as well as on write: it
		// would otherwise be read from outside the repo and written outside
		// the staging dir
		Convey("with an escaping path it errors", func() {
			for _, path := range []string{
				"../outside.txt",
				"../../../tmp/evil.sh",
				"usr/../../escape.sh",
				"/tmp/absolute.sh",
				"DEBIAN/postinst",
				".",
			} {
				mf := filepath.Join(dir, "md5sums")
				writeFile(t, mf, hiSum+"  "+path+"\n")

				_, err := ReadManifest(mf)
				So(err, ShouldNotBeNil)
			}
		})

		Convey("with a path that only looks like an escape it is kept", func() {
			mf := filepath.Join(dir, "md5sums")
			writeFile(t, mf, hiSum+"  usr/lib/..foo/bar\n")

			m, err := ReadManifest(mf)
			So(err, ShouldBeNil)
			So(len(m), ShouldEqual, 1)
			So(m[0].Path, ShouldEqual, "usr/lib/..foo/bar")
		})

		Convey("a leading ./ is normalized away", func() {
			mf := filepath.Join(dir, "md5sums")
			writeFile(t, mf, hiSum+"  ./bin/thing\n")

			m, err := ReadManifest(mf)
			So(err, ShouldBeNil)
			So(m[0].Path, ShouldEqual, "bin/thing")
		})
	})
}

func TestReadDocFilesRejectsEscapingPaths(t *testing.T) {
	Convey("given a docfiles list", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-docfiles")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		// a doc file's source is read from the repo and copied to the doc dir,
		// so an escaping source would pull an arbitrary file into the package
		// while every recorded sum still matched
		Convey("an escaping source errors", func() {
			for _, path := range []string{
				"../secret.txt",
				"/etc/shadow",
				"docs/../../escape.md",
			} {
				df := filepath.Join(dir, "docfiles")
				writeFile(t, df, path+"\n")

				_, err := ReadDocFiles(df)
				So(err, ShouldNotBeNil)
			}
		})

		Convey("ordinary sources and comments are kept", func() {
			df := filepath.Join(dir, "docfiles")
			writeFile(t, df, "# a comment\n\nREADME.md\ndocs/guide.md\n")

			d, err := ReadDocFiles(df)
			So(err, ShouldBeNil)
			So(len(d), ShouldEqual, 2)
			So(d[0], ShouldEqual, "README.md")
			So(d[1], ShouldEqual, "docs/guide.md")
		})
	})
}

func TestConfine(t *testing.T) {
	Convey("given a root directory", t, func() {
		Convey("a path inside it resolves", func() {
			path, err := confine("/tmp/root", "usr/bin/thing")
			So(err, ShouldBeNil)
			So(path, ShouldEqual, "/tmp/root/usr/bin/thing")
		})

		Convey("a path escaping it errors", func() {
			for _, rel := range []string{"../evil", "usr/../../evil", "../../etc/shadow"} {
				_, err := confine("/tmp/root", rel)
				So(err, ShouldNotBeNil)
			}
		})

		// Join treats an absolute path as relative to the root, so it is
		// contained rather than rejected; manifestPath rejects it outright
		Convey("an absolute path is contained under the root", func() {
			path, err := confine("/tmp/root", "/etc/shadow")
			So(err, ShouldBeNil)
			So(path, ShouldEqual, "/tmp/root/etc/shadow")
		})
	})
}

func TestManifestWrite(t *testing.T) {
	Convey("given an unsorted manifest", t, func() {
		m := Manifest{
			{Sum: "def", Path: "etc/conf"},
			{Sum: "abc", Path: "bin/thing"},
		}

		var buf bytes.Buffer
		_, err := m.Write(&buf)
		So(err, ShouldBeNil)

		Convey("it writes standard Debian format sorted by path", func() {
			So(buf.String(), ShouldEqual, "abc  bin/thing\ndef  etc/conf\n")
		})
	})
}

func TestVerify(t *testing.T) {
	Convey("given an initialized package with a registered file", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-verify")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)
		So(Initialize(dir), ShouldBeNil)

		writeFile(t, filepath.Join(dir, "bin", "thing"), "hi\n")
		p, err := NewPackage(dir)
		So(err, ShouldBeNil)
		So(p.AddFile("bin/thing"), ShouldBeNil)

		Convey("the manifest holds the correct sum", func() {
			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(len(m), ShouldEqual, 1)
			So(m[0].Path, ShouldEqual, "bin/thing")
			So(m[0].Sum, ShouldEqual, hiSum)
		})

		Convey("verify passes when the file is unchanged", func() {
			problems, err := p.Verify(false)
			So(err, ShouldBeNil)
			So(problems, ShouldBeEmpty)
		})

		Convey("verify fails on a mismatch", func() {
			writeFile(t, filepath.Join(dir, "bin", "thing"), "changed\n")
			problems, err := p.Verify(false)
			So(err, ShouldNotBeNil)
			So(len(problems), ShouldEqual, 1)

			Convey("but only warns when insecure", func() {
				problems, err := p.Verify(true)
				So(err, ShouldBeNil)
				So(len(problems), ShouldEqual, 1)
			})
		})

		Convey("verify fails when a file is missing", func() {
			So(os.Remove(filepath.Join(dir, "bin", "thing")), ShouldBeNil)
			problems, err := p.Verify(false)
			So(err, ShouldNotBeNil)
			So(len(problems), ShouldEqual, 1)
		})
	})
}
