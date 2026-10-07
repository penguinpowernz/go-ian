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
