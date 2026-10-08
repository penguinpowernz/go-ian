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

func TestSum(t *testing.T) {
	Convey("given a file", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-sum")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		Convey("its md5 is returned as lowercase hex", func() {
			path := filepath.Join(dir, "f")
			writeFile(t, path, "hi\n")

			sum, err := Sum(path)
			So(err, ShouldBeNil)
			So(sum, ShouldEqual, hiSum)
		})

		Convey("an empty file has the empty md5", func() {
			path := filepath.Join(dir, "empty")
			writeFile(t, path, "")

			sum, err := Sum(path)
			So(err, ShouldBeNil)
			So(sum, ShouldEqual, "d41d8cd98f00b204e9800998ecf8427e")
		})

		Convey("a missing file errors", func() {
			_, err := Sum(filepath.Join(dir, "nope"))
			So(err, ShouldNotBeNil)
		})
	})
}

func TestManifestPaths(t *testing.T) {
	Convey("given a manifest", t, func() {
		m := Manifest{{Sum: "abc", Path: "bin/thing"}, {Sum: "def", Path: "etc/conf"}}

		Convey("its paths are returned in order", func() {
			So(m.Paths(), ShouldResemble, []string{"bin/thing", "etc/conf"})
		})

		Convey("an empty manifest gives an empty list", func() {
			So(Manifest{}.Paths(), ShouldBeEmpty)
		})
	})
}

func TestManifestPathNormalization(t *testing.T) {
	Convey("given a path to normalize", t, func() {
		Convey("ordinary relative paths pass through cleaned", func() {
			for in, want := range map[string]string{
				"bin/thing":       "bin/thing",
				"./bin/thing":     "bin/thing",
				"bin//thing":      "bin/thing",
				"usr/bin/":        "usr/bin",
				"usr/./bin/thing": "usr/bin/thing",
				"usr/lib/../bin":  "usr/bin",
				"README.md":       "README.md",
			} {
				got, err := manifestPath(in)
				So(err, ShouldBeNil)
				So(got, ShouldEqual, want)
			}
		})

		Convey("paths that can never belong in the manifest are rejected", func() {
			for _, in := range []string{
				"/usr/bin/thing", // absolute
				"../outside",     // escapes
				"usr/../../out",  // escapes after cleaning
				"..",
				".",
				"DEBIAN",          // control files are staged separately
				"DEBIAN/postinst", // and md5sums could never sum itself
				"DEBIAN/md5sums",
			} {
				_, err := manifestPath(in)
				So(err, ShouldNotBeNil)
			}
		})

		// a directory merely named like an escape is a perfectly good path
		Convey("a path that only looks like an escape is kept", func() {
			got, err := manifestPath("usr/..lib/thing")
			So(err, ShouldBeNil)
			So(got, ShouldEqual, "usr/..lib/thing")
		})

		// only the control dir itself is off limits, not anything starting
		// with those letters
		Convey("a directory merely prefixed DEBIAN is kept", func() {
			got, err := manifestPath("DEBIANISH/thing")
			So(err, ShouldBeNil)
			So(got, ShouldEqual, "DEBIANISH/thing")
		})
	})
}

func TestRelToDir(t *testing.T) {
	Convey("given a package", t, func() {
		p, dir := newPkg(t)

		Convey("an absolute path inside it is made relative", func() {
			rel, err := p.relToDir(filepath.Join(dir, "usr", "bin", "app"))
			So(err, ShouldBeNil)
			So(rel, ShouldEqual, "usr/bin/app")
		})

		// relative paths are taken against the package dir rather than the
		// working directory, so `ian -d some/pkg add .` means that package
		Convey("a relative path is taken against the package dir", func() {
			rel, err := p.relToDir("usr/bin/app")
			So(err, ShouldBeNil)
			So(rel, ShouldEqual, "usr/bin/app")
		})

		Convey("the package dir itself is \".\"", func() {
			rel, err := p.relToDir(dir)
			So(err, ShouldBeNil)
			So(rel, ShouldEqual, ".")
		})

		Convey("a path outside the package errors", func() {
			_, err := p.relToDir("../outside")
			So(err, ShouldNotBeNil)

			_, err = p.relToDir("/etc/shadow")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestExpandFiles(t *testing.T) {
	Convey("given a package with a tree of files", t, func() {
		p, dir := newPkg(t)

		writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "hi\n")
		writeFile(t, filepath.Join(dir, "usr", "share", "data"), "hi\n")
		writeFile(t, filepath.Join(dir, "etc", "app.conf"), "hi\n")

		Convey("a directory is walked recursively", func() {
			found, err := p.ExpandFiles([]string{"usr"})
			So(err, ShouldBeNil)
			So(found, ShouldResemble, []string{"usr/bin/app", "usr/share/data"})
		})

		Convey("a plain file passes through", func() {
			found, err := p.ExpandFiles([]string{"etc/app.conf"})
			So(err, ShouldBeNil)
			So(found, ShouldResemble, []string{"etc/app.conf"})
		})

		Convey("results are sorted and deduplicated across arguments", func() {
			found, err := p.ExpandFiles([]string{"usr/bin/app", "usr", "etc"})
			So(err, ShouldBeNil)
			So(found, ShouldResemble, []string{"etc/app.conf", "usr/bin/app", "usr/share/data"})
		})

		// `ian add .` has to behave sensibly, which means quietly skipping
		// the things that can never be package contents
		Convey("expanding the whole package skips what cannot belong", func() {
			writeFile(t, filepath.Join(dir, "pkg", "old.deb"), "hi\n")
			writeFile(t, filepath.Join(dir, ".git", "config"), "hi\n")
			writeFile(t, filepath.Join(dir, ".gitignore"), "hi\n")
			writeFile(t, filepath.Join(dir, ".gitkeep"), "hi\n")
			writeFile(t, filepath.Join(dir, ".ianignore"), "hi\n")
			writeFile(t, filepath.Join(dir, p.DocDir(), "guide.md"), "hi\n")

			found, err := p.ExpandFiles([]string{"."})
			So(err, ShouldBeNil)

			So(found, ShouldContain, "usr/bin/app")
			So(found, ShouldContain, "etc/app.conf")

			for _, unwanted := range []string{
				"DEBIAN/control",
				"DEBIAN/md5sums",
				"pkg/old.deb",
				".git/config",
				".gitignore",
				".gitkeep",
				".ianignore",
				filepath.Join(p.DocDir(), "guide.md"),
			} {
				So(found, ShouldNotContain, unwanted)
			}
		})

		// a symlink cannot be summed or shipped meaningfully, and following
		// one would package whatever it resolves to
		Convey("symlinks are not included", func() {
			link := filepath.Join(dir, "etc", "link.conf")
			if err := os.Symlink(filepath.Join(dir, "usr", "bin", "app"), link); err == nil {
				found, err := p.ExpandFiles([]string{"etc"})
				So(err, ShouldBeNil)
				So(found, ShouldResemble, []string{"etc/app.conf"})
			}
		})

		Convey("a path outside the package errors", func() {
			_, err := p.ExpandFiles([]string{"../outside"})
			So(err, ShouldNotBeNil)
		})

		Convey("a path that does not exist errors", func() {
			_, err := p.ExpandFiles([]string{"nope"})
			So(err, ShouldNotBeNil)
		})
	})
}

func TestAddFile(t *testing.T) {
	Convey("given an initialized package", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "hi\n")

		Convey("adding a file registers its sum", func() {
			So(p.AddFile("usr/bin/app"), ShouldBeNil)

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(len(m), ShouldEqual, 1)
			So(m[0].Path, ShouldEqual, "usr/bin/app")
			So(m[0].Sum, ShouldEqual, hiSum)
		})

		// re-adding is how a changed file gets its sum refreshed, so it must
		// update the entry rather than append a second one
		Convey("adding it again updates the sum in place", func() {
			So(p.AddFile("usr/bin/app"), ShouldBeNil)
			writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "changed\n")
			So(p.AddFile("usr/bin/app"), ShouldBeNil)

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(len(m), ShouldEqual, 1)
			So(m[0].Sum, ShouldNotEqual, hiSum)

			problems, err := p.Verify(false)
			So(err, ShouldBeNil)
			So(problems, ShouldBeEmpty)
		})

		Convey("a leading ./ is normalized before registering", func() {
			So(p.AddFile("./usr/bin/app"), ShouldBeNil)

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m[0].Path, ShouldEqual, "usr/bin/app")
		})

		Convey("a missing file errors", func() {
			So(p.AddFile("usr/bin/nope"), ShouldNotBeNil)
		})

		Convey("a control file cannot be registered", func() {
			So(p.AddFile("DEBIAN/postinst"), ShouldNotBeNil)
			So(p.AddFile("DEBIAN/md5sums"), ShouldNotBeNil)
		})

		Convey("an escaping path cannot be registered", func() {
			So(p.AddFile("../outside"), ShouldNotBeNil)
			So(p.AddFile("/etc/shadow"), ShouldNotBeNil)
		})
	})
}

func TestRemoveFiles(t *testing.T) {
	Convey("given a package with several registered files", t, func() {
		p, dir := newPkg(t)

		for _, f := range []string{"usr/bin/app", "usr/share/data", "etc/app.conf"} {
			writeFile(t, filepath.Join(dir, f), "hi\n")
			So(p.AddFile(f), ShouldBeNil)
		}

		Convey("removing one file unregisters just that one", func() {
			removed, err := p.RemoveFiles([]string{"usr/bin/app"})
			So(err, ShouldBeNil)
			So(removed, ShouldResemble, []string{"usr/bin/app"})

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m.Paths(), ShouldResemble, []string{"etc/app.conf", "usr/share/data"})

			// unregistering only means the file is no longer packaged
			Convey("and leaves the file on disk", func() {
				So(fexists(filepath.Join(dir, "usr", "bin", "app")), ShouldBeTrue)
			})
		})

		Convey("removing a directory unregisters everything beneath it", func() {
			removed, err := p.RemoveFiles([]string{"usr"})
			So(err, ShouldBeNil)
			So(removed, ShouldResemble, []string{"usr/bin/app", "usr/share/data"})

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m.Paths(), ShouldResemble, []string{"etc/app.conf"})
		})

		Convey("removing \".\" unregisters the whole package", func() {
			removed, err := p.RemoveFiles([]string{"."})
			So(err, ShouldBeNil)
			So(len(removed), ShouldEqual, 3)

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m, ShouldBeEmpty)
		})

		// matching is done against the manifest rather than the disk, so a
		// file already deleted can still be unregistered
		Convey("a file already deleted from disk can still be unregistered", func() {
			So(os.Remove(filepath.Join(dir, "usr", "bin", "app")), ShouldBeNil)

			removed, err := p.RemoveFiles([]string{"usr/bin/app"})
			So(err, ShouldBeNil)
			So(removed, ShouldResemble, []string{"usr/bin/app"})
		})

		Convey("an argument matching nothing errors", func() {
			_, err := p.RemoveFiles([]string{"usr/bin/nope"})
			So(err, ShouldNotBeNil)

			Convey("and nothing is removed", func() {
				m, err := p.Manifest()
				So(err, ShouldBeNil)
				So(len(m), ShouldEqual, 3)
			})
		})

		Convey("an escaping path errors", func() {
			_, err := p.RemoveFiles([]string{"../outside"})
			So(err, ShouldNotBeNil)
		})
	})
}

func TestStatus(t *testing.T) {
	Convey("given a package with registered files", t, func() {
		p, dir := newPkg(t)

		writeFile(t, filepath.Join(dir, "usr", "bin", "ok"), "hi\n")
		writeFile(t, filepath.Join(dir, "usr", "bin", "changed"), "hi\n")
		writeFile(t, filepath.Join(dir, "usr", "bin", "gone"), "hi\n")
		for _, f := range []string{"usr/bin/ok", "usr/bin/changed", "usr/bin/gone"} {
			So(p.AddFile(f), ShouldBeNil)
		}

		writeFile(t, filepath.Join(dir, "usr", "bin", "changed"), "different\n")
		So(os.Remove(filepath.Join(dir, "usr", "bin", "gone")), ShouldBeNil)

		statuses, err := p.Status()
		So(err, ShouldBeNil)
		So(len(statuses), ShouldEqual, 3)

		byPath := map[string]FileStatus{}
		for _, st := range statuses {
			byPath[st.Path] = st
		}

		Convey("an unchanged file is OK and reports no problem", func() {
			st := byPath["usr/bin/ok"]
			So(st.State, ShouldEqual, StateOK)
			So(st.Got, ShouldEqual, hiSum)
			So(st.Problem(), ShouldEqual, "")
			So(st.IsDoc(), ShouldBeFalse)
		})

		Convey("a changed file reports both sums", func() {
			st := byPath["usr/bin/changed"]
			So(st.State, ShouldEqual, StateModified)
			So(st.Want, ShouldEqual, hiSum)
			So(st.Got, ShouldNotEqual, hiSum)
			So(st.Problem(), ShouldContainSubstring, "md5 mismatch")
		})

		Convey("a deleted file is reported as missing", func() {
			st := byPath["usr/bin/gone"]
			So(st.State, ShouldEqual, StateMissing)
			So(st.Got, ShouldEqual, "")
			So(st.Problem(), ShouldContainSubstring, "missing from repo")
		})

		// Status reports drift without failing, so a caller can show it
		Convey("it never errors on drift, unlike Verify", func() {
			_, err := p.Status()
			So(err, ShouldBeNil)
		})
	})
}

func TestFileStatusIsDocAndProblem(t *testing.T) {
	Convey("given a doc file status", t, func() {
		st := FileStatus{
			Path:   "usr/share/doc/app/guide.md",
			Source: "docs/guide.md",
			State:  StateModified,
			Want:   "aaa",
			Got:    "bbb",
		}

		Convey("it knows it is a doc file", func() {
			So(st.IsDoc(), ShouldBeTrue)
		})

		// the repo file is the one to go and look at, but the install path
		// is what the user registered, so both are named
		Convey("the problem names the source and the install path", func() {
			So(st.Problem(), ShouldContainSubstring, "docs/guide.md")
			So(st.Problem(), ShouldContainSubstring, "installs as usr/share/doc/app/guide.md")
		})

		Convey("an error state reports the error", func() {
			st.State = StateError
			st.Err = os.ErrPermission
			So(st.Problem(), ShouldContainSubstring, os.ErrPermission.Error())
		})

		Convey("a status with no source at all falls back to the path", func() {
			st := FileStatus{Path: "usr/bin/app", State: StateMissing}
			So(st.IsDoc(), ShouldBeFalse)
			So(st.Problem(), ShouldContainSubstring, "usr/bin/app")
		})
	})
}

func TestWriteManifestRoundTrips(t *testing.T) {
	Convey("given a package", t, func() {
		p, _ := newPkg(t)

		Convey("a written manifest reads back the same, sorted", func() {
			So(p.WriteManifest(Manifest{
				{Sum: "def", Path: "etc/conf"},
				{Sum: "abc", Path: "bin/thing"},
			}), ShouldBeNil)

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m, ShouldResemble, Manifest{
				{Sum: "abc", Path: "bin/thing"},
				{Sum: "def", Path: "etc/conf"},
			})
		})

		Convey("writing replaces the previous contents entirely", func() {
			So(p.WriteManifest(Manifest{{Sum: "abc", Path: "bin/thing"}}), ShouldBeNil)
			So(p.WriteManifest(Manifest{{Sum: "def", Path: "etc/conf"}}), ShouldBeNil)

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m.Paths(), ShouldResemble, []string{"etc/conf"})
		})

		Convey("writing an empty manifest empties the file", func() {
			So(p.WriteManifest(Manifest{{Sum: "abc", Path: "bin/thing"}}), ShouldBeNil)
			So(p.WriteManifest(Manifest{}), ShouldBeNil)

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m, ShouldBeEmpty)
		})
	})
}

func TestUpdateFiles(t *testing.T) {
	Convey("given a package with drifted files", t, func() {
		p, dir := newPkg(t)

		writeFile(t, filepath.Join(dir, "usr", "bin", "ok"), "hi\n")
		writeFile(t, filepath.Join(dir, "usr", "bin", "changed"), "hi\n")
		writeFile(t, filepath.Join(dir, "usr", "bin", "also"), "hi\n")
		for _, f := range []string{"usr/bin/ok", "usr/bin/changed", "usr/bin/also"} {
			So(p.AddFile(f), ShouldBeNil)
		}

		writeFile(t, filepath.Join(dir, "usr", "bin", "changed"), "different\n")
		writeFile(t, filepath.Join(dir, "usr", "bin", "also"), "different too\n")

		Convey("it re-sums every modified file at once", func() {
			updated, problems, err := p.UpdateFiles()
			So(err, ShouldBeNil)
			So(problems, ShouldBeEmpty)
			So(updated, ShouldResemble, []string{"usr/bin/also", "usr/bin/changed"})

			// the manifest now matches the repo, so verification passes
			_, err = p.Verify(false)
			So(err, ShouldBeNil)
		})

		Convey("it leaves unchanged files' entries alone", func() {
			_, _, err := p.UpdateFiles()
			So(err, ShouldBeNil)

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			byPath := map[string]string{}
			for _, e := range m {
				byPath[e.Path] = e.Sum
			}
			So(byPath["usr/bin/ok"], ShouldEqual, hiSum)
			So(byPath["usr/bin/changed"], ShouldNotEqual, hiSum)
		})

		Convey("it registers nothing new", func() {
			writeFile(t, filepath.Join(dir, "usr", "bin", "unregistered"), "hi\n")

			_, _, err := p.UpdateFiles()
			So(err, ShouldBeNil)

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(len(m), ShouldEqual, 3)
			So(m.Paths(), ShouldNotContain, "usr/bin/unregistered")
		})

		// a missing file needs restoring or "ian rm", so -u must not quietly
		// drop it from the package
		Convey("a missing file is reported and left registered", func() {
			So(os.Remove(filepath.Join(dir, "usr", "bin", "ok")), ShouldBeNil)

			updated, problems, err := p.UpdateFiles()
			So(err, ShouldBeNil)
			So(updated, ShouldNotContain, "usr/bin/ok")
			So(len(problems), ShouldEqual, 1)
			So(problems[0], ShouldContainSubstring, "missing from repo")

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m.Paths(), ShouldContain, "usr/bin/ok")
		})
	})

	Convey("given a package whose files all match", t, func() {
		p, dir := newPkg(t)

		writeFile(t, filepath.Join(dir, "usr", "bin", "ok"), "hi\n")
		So(p.AddFile("usr/bin/ok"), ShouldBeNil)

		Convey("it updates nothing", func() {
			updated, problems, err := p.UpdateFiles()
			So(err, ShouldBeNil)
			So(updated, ShouldBeEmpty)
			So(problems, ShouldBeEmpty)
		})
	})

	// doc files are staged from a different path than they install to, so the
	// new sum has to come from the source file in the repo
	Convey("given a drifted doc file", t, func() {
		p, dir := newPkg(t)

		writeFile(t, filepath.Join(dir, "docs", "guide.md"), "hi\n")
		So(p.AddDocFile("docs/guide.md"), ShouldBeNil)

		writeFile(t, filepath.Join(dir, "docs", "guide.md"), "updated\n")

		Convey("it re-sums it from its source", func() {
			updated, problems, err := p.UpdateFiles()
			So(err, ShouldBeNil)
			So(problems, ShouldBeEmpty)
			So(updated, ShouldResemble, []string{p.DocDest("docs/guide.md")})

			_, err = p.Verify(false)
			So(err, ShouldBeNil)
		})
	})
}

func TestAddFileAllArches(t *testing.T) {
	Convey("given a package with per-arch manifests", t, func() {
		p, dir := newPkg(t)
		mf := func(n string) string { return filepath.Join(dir, "DEBIAN", n) }

		// two arches each carrying their own binary sum, the way a
		// cross-compiled package looks once both have been registered
		writeFile(t, mf("md5sums.amd64"), "aaa  usr/bin/app\n")
		writeFile(t, mf("md5sums.arm64"), "bbb  usr/bin/app\n")
		writeFile(t, filepath.Join(dir, "etc", "app.conf"), "hi\n")

		p.ctrl.Arch = "amd64"

		Convey("adding a file registers it in every manifest", func() {
			written, err := p.AddFileAllArches("etc/app.conf")
			So(err, ShouldBeNil)
			So(written, ShouldResemble, []string{"md5sums", "md5sums.amd64", "md5sums.arm64"})

			for _, n := range []string{"md5sums", "md5sums.amd64", "md5sums.arm64"} {
				m, err := ReadManifest(mf(n))
				So(err, ShouldBeNil)

				var got string
				for _, e := range m {
					if e.Path == "etc/app.conf" {
						got = e.Sum
					}
				}
				So(got, ShouldEqual, hiSum)
			}
		})

		// the whole point of per-arch manifests is that the same path holds
		// different bytes per arch, so the other entries must be untouched
		Convey("the other arches' existing sums are left alone", func() {
			_, err := p.AddFileAllArches("etc/app.conf")
			So(err, ShouldBeNil)

			for n, want := range map[string]string{"md5sums.amd64": "aaa", "md5sums.arm64": "bbb"} {
				m, err := ReadManifest(mf(n))
				So(err, ShouldBeNil)

				var got string
				for _, e := range m {
					if e.Path == "usr/bin/app" {
						got = e.Sum
					}
				}
				So(got, ShouldEqual, want)
			}
		})

		Convey("adding it again updates the sum in place in every manifest", func() {
			_, err := p.AddFileAllArches("etc/app.conf")
			So(err, ShouldBeNil)

			writeFile(t, filepath.Join(dir, "etc", "app.conf"), "changed\n")
			_, err = p.AddFileAllArches("etc/app.conf")
			So(err, ShouldBeNil)

			for _, n := range []string{"md5sums", "md5sums.amd64", "md5sums.arm64"} {
				m, err := ReadManifest(mf(n))
				So(err, ShouldBeNil)

				count := 0
				for _, e := range m {
					if e.Path == "etc/app.conf" {
						count++
						So(e.Sum, ShouldNotEqual, hiSum)
					}
				}
				So(count, ShouldEqual, 1)
			}
		})

		Convey("a missing file errors", func() {
			_, err := p.AddFileAllArches("etc/nope.conf")
			So(err, ShouldNotBeNil)
		})

		Convey("a control file cannot be registered", func() {
			_, err := p.AddFileAllArches("DEBIAN/control")
			So(err, ShouldNotBeNil)
		})

		Convey("an escaping path cannot be registered", func() {
			_, err := p.AddFileAllArches("../outside")
			So(err, ShouldNotBeNil)
		})
	})

	// a package that has never been through "ian set -a" has only the plain
	// manifest, and -a must still register there rather than doing nothing
	Convey("given a single-arch package", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, "etc", "app.conf"), "hi\n")
		p.ctrl.Arch = "amd64"

		Convey("adding a file registers it in the plain manifest", func() {
			written, err := p.AddFileAllArches("etc/app.conf")
			So(err, ShouldBeNil)
			So(written, ShouldResemble, []string{"md5sums"})

			m, err := ReadManifest(filepath.Join(dir, "DEBIAN", "md5sums"))
			So(err, ShouldBeNil)
			So(len(m), ShouldEqual, 1)
			So(m[0].Sum, ShouldEqual, hiSum)
		})
	})
}
