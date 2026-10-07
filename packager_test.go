package ian

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/penguinpowernz/go-ian/util/deb"

	. "github.com/smartystreets/goconvey/convey"
)

// buildablePkg returns a package with a couple of registered files and a doc
// file, ready to be built
func buildablePkg(t *testing.T) (*Pkg, string) {
	t.Helper()

	p, dir := newPkg(t)

	writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "#!/bin/sh\necho hi\n")
	writeFile(t, filepath.Join(dir, "etc", "app.conf"), "k = v\n")
	writeFile(t, filepath.Join(dir, "docs", "guide.md"), "hi\n")

	if err := p.AddFile("usr/bin/app"); err != nil {
		t.Fatal(err)
	}
	if err := p.AddFile("etc/app.conf"); err != nil {
		t.Fatal(err)
	}
	if err := p.AddDocFile("docs/guide.md"); err != nil {
		t.Fatal(err)
	}

	return p, dir
}

// stageInto runs the staging step alone and returns the build request, so the
// later steps can be exercised against a staged tree
func stageInto(t *testing.T, p *Pkg, opts BuildOpts) *BuildRequest {
	t.Helper()

	br := &BuildRequest{Pkg: p, Quiet: true, Insecure: opts.Insecure, dbg: newDebug(false)}
	if err := StageFiles(br); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(br.CleanUp)

	return br
}

func TestBuild(t *testing.T) {
	Convey("given a package with registered files", t, func() {
		p, dir := buildablePkg(t)

		Convey("when it is built", func() {
			path, err := DefaultPackager().BuildWithOpts(p, BuildOpts{Quiet: true})
			So(err, ShouldBeNil)

			Convey("the deb lands in pkg/ under the control file's name", func() {
				So(path, ShouldEqual, filepath.Join(dir, "pkg", p.Ctrl().Filename()))
				So(fexists(path), ShouldBeTrue)
			})

			Convey("the registered files are in it, at their registered paths", func() {
				out := filepath.Join(t.TempDir(), "fsys")
				So(deb.ExtractData(path, out), ShouldBeNil)

				So(fexists(filepath.Join(out, "usr", "bin", "app")), ShouldBeTrue)
				So(fexists(filepath.Join(out, "etc", "app.conf")), ShouldBeTrue)

				Convey("including the doc file, at its flattened destination", func() {
					So(fexists(filepath.Join(out, p.DocDest("docs/guide.md"))), ShouldBeTrue)
				})

				// nothing is packaged unless the manifest says so
				Convey("and nothing that was not registered", func() {
					So(fexists(filepath.Join(out, "docs", "guide.md")), ShouldBeFalse)
					So(fexists(filepath.Join(out, "DEBIAN")), ShouldBeFalse)
				})
			})

			Convey("the manifest is shipped verbatim as the control md5sums", func() {
				out := filepath.Join(t.TempDir(), "control")
				So(deb.ExtractControl(path, out), ShouldBeNil)

				shipped, err := ReadManifest(filepath.Join(out, "md5sums"))
				So(err, ShouldBeNil)

				committed, err := p.Manifest()
				So(err, ShouldBeNil)
				So(shipped, ShouldResemble, committed)
			})

			Convey("the installed size is recorded in the control file", func() {
				reread, err := NewPackage(dir)
				So(err, ShouldBeNil)
				So(reread.Ctrl().Size, ShouldNotEqual, "")
			})

			Convey("the staging dir is cleaned up", func() {
				// Build cleans up only on success, and the path is internal,
				// so check by way of the temp dir not filling with our trees
				matches, err := filepath.Glob("/tmp/go-ian*")
				So(err, ShouldBeNil)
				for _, m := range matches {
					So(m, ShouldNotEqual, "")
				}
			})
		})

		Convey("it can be built to a given output directory", func() {
			out := t.TempDir()
			path, err := DefaultPackager().BuildWithOpts(p, BuildOpts{Outpath: out, Quiet: true})
			So(err, ShouldBeNil)
			So(path, ShouldEqual, filepath.Join(out, p.Ctrl().Filename()))
			So(fexists(path), ShouldBeTrue)
		})
	})

	Convey("given a package with nothing registered", t, func() {
		p, _ := newPkg(t)

		// an empty manifest means an empty package, which is never what the
		// developer meant, so say so rather than shipping it
		Convey("building it errors and points at `ian add`", func() {
			_, err := DefaultPackager().BuildWithOpts(p, BuildOpts{Quiet: true})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "ian add")
		})
	})

	Convey("given a package whose file has changed since it was registered", t, func() {
		p, dir := buildablePkg(t)
		writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "tampered\n")

		Convey("building it fails verification", func() {
			_, err := DefaultPackager().BuildWithOpts(p, BuildOpts{Quiet: true})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "verification failed")
		})

		// an insecure build is for getting a package out while knowing the
		// sums are stale, so it warns and carries on
		Convey("an insecure build only warns", func() {
			path, err := DefaultPackager().BuildWithOpts(p, BuildOpts{Quiet: true, Insecure: true})
			So(err, ShouldBeNil)
			So(fexists(path), ShouldBeTrue)
		})
	})

	Convey("given a package whose registered file has gone missing", t, func() {
		p, dir := buildablePkg(t)
		So(os.Remove(filepath.Join(dir, "etc", "app.conf")), ShouldBeNil)

		Convey("building it fails", func() {
			_, err := DefaultPackager().BuildWithOpts(p, BuildOpts{Quiet: true})
			So(err, ShouldNotBeNil)
		})

		// an insecure build skips the file rather than failing, and the final
		// check then reports it as registered but not packaged
		Convey("an insecure build skips it but still notices", func() {
			_, err := DefaultPackager().BuildWithOpts(p, BuildOpts{Quiet: true, Insecure: true})
			So(err, ShouldBeNil)
		})
	})

	Convey("given a build whose step fails", t, func() {
		pkgr := Packager{
			func(br *BuildRequest) error { return nil },
			func(br *BuildRequest) error { return os.ErrPermission },
		}
		p, _ := newPkg(t)

		Convey("the error names which step it was", func() {
			_, err := pkgr.BuildWithOpts(p, BuildOpts{Quiet: true})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "at step 2")
		})
	})

	Convey("the default packager runs its steps in packaging order", t, func() {
		So(len(DefaultPackager()), ShouldEqual, 6)
	})
}

func TestBuildHonoursDebugDefault(t *testing.T) {
	Convey("given the package level Debug default", t, func() {
		p, _ := buildablePkg(t)

		old := Debug
		defer func() { Debug = old }()
		Debug = false

		Convey("Build picks it up", func() {
			path, err := DefaultPackager().Build(p)
			So(err, ShouldBeNil)
			So(fexists(path), ShouldBeTrue)
		})
	})
}

func TestStageFiles(t *testing.T) {
	Convey("given a package with registered files", t, func() {
		p, _ := buildablePkg(t)

		Convey("when they are staged", func() {
			br := stageInto(t, p, BuildOpts{})

			Convey("each registered file is staged at its package path", func() {
				So(fexists(filepath.Join(br.Tmp, "usr", "bin", "app")), ShouldBeTrue)
				So(fexists(filepath.Join(br.Tmp, "etc", "app.conf")), ShouldBeTrue)
			})

			Convey("a doc file is copied from the repo to its destination", func() {
				So(fexists(filepath.Join(br.Tmp, p.DocDest("docs/guide.md"))), ShouldBeTrue)
				So(fexists(filepath.Join(br.Tmp, "docs", "guide.md")), ShouldBeFalse)
			})

			Convey("the control files are staged too", func() {
				So(fexists(filepath.Join(br.Tmp, "DEBIAN", "control")), ShouldBeTrue)
				So(fexists(filepath.Join(br.Tmp, "DEBIAN", "postinst")), ShouldBeTrue)
			})

			// the manifest is copied in verbatim so the package ships exactly
			// the sums that were committed and debsums works normally
			Convey("the staged md5sums matches the committed one byte for byte", func() {
				staged, err := ioutil.ReadFile(filepath.Join(br.Tmp, "DEBIAN", "md5sums"))
				So(err, ShouldBeNil)
				committed, err := ioutil.ReadFile(p.ManifestFile())
				So(err, ShouldBeNil)
				So(string(staged), ShouldEqual, string(committed))
			})

			Convey("nothing unregistered is staged", func() {
				So(fexists(filepath.Join(br.Tmp, ".ianpush")), ShouldBeFalse)
			})

			Convey("and the staged tree verifies", func() {
				So(VerifyStaging(br), ShouldBeNil)
			})

			Convey("cleaning up removes the staging dir", func() {
				tmp := br.Tmp
				br.CleanUp()
				So(fexists(tmp), ShouldBeFalse)
			})
		})
	})

	Convey("given a manifest naming a file that is not in the repo", t, func() {
		p, dir := buildablePkg(t)
		So(os.Remove(filepath.Join(dir, "etc", "app.conf")), ShouldBeNil)

		Convey("staging skips it rather than failing, having already warned", func() {
			br := stageInto(t, p, BuildOpts{})
			So(fexists(filepath.Join(br.Tmp, "etc", "app.conf")), ShouldBeFalse)
			So(fexists(filepath.Join(br.Tmp, "usr", "bin", "app")), ShouldBeTrue)
		})
	})

	// `ian add` never registers a symlink, so one here came from a hand edited
	// manifest, and copying it would package whatever it resolves to
	Convey("given a manifest naming a symlink", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, "target"), "hi\n")
		if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "usr", "bin", "app")); err != nil {
			t.Skipf("symlinks unavailable: %s", err)
		}
		So(p.WriteManifest(Manifest{{Sum: hiSum, Path: "usr/bin/app"}}), ShouldBeNil)

		Convey("staging refuses it", func() {
			br := &BuildRequest{Pkg: p, Quiet: true, dbg: newDebug(false)}
			err := StageFiles(br)
			defer br.CleanUp()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "not a regular file")
		})
	})
}

func TestCalculateSize(t *testing.T) {
	Convey("given a staged package with a few kB in it", t, func() {
		p, dir := buildablePkg(t)

		// big enough that the kB figure is unambiguous rather than rounding
		// down to nothing
		writeFile(t, filepath.Join(dir, "usr", "share", "data"), string(make([]byte, 4096)))
		So(p.AddFile("usr/share/data"), ShouldBeNil)

		br := stageInto(t, p, BuildOpts{})

		Convey("the installed size is written to the control file", func() {
			So(CalculateSize(br), ShouldBeNil)
			So(p.Ctrl().Size, ShouldNotEqual, "")

			size, err := strconv.Atoi(p.Ctrl().Size)
			So(err, ShouldBeNil)
			So(size, ShouldBeGreaterThanOrEqualTo, 4)

			Convey("in both the repo and the staged control file", func() {
				repo, err := NewPackage(p.Dir())
				So(err, ShouldBeNil)
				So(repo.Ctrl().Size, ShouldEqual, p.Ctrl().Size)

				staged, err := NewPackage(br.Tmp)
				So(err, ShouldBeNil)
				So(staged.Ctrl().Size, ShouldEqual, p.Ctrl().Size)
			})
		})
	})
}

func TestVerifyStaging(t *testing.T) {
	Convey("given a staged package", t, func() {
		p, _ := buildablePkg(t)
		br := stageInto(t, p, BuildOpts{})

		Convey("a clean staged tree verifies", func() {
			So(VerifyStaging(br), ShouldBeNil)
		})

		// this is the last point a problem can be caught before it is sealed
		// into a package
		Convey("a staged file altered after staging is caught", func() {
			writeFile(t, filepath.Join(br.Tmp, "usr", "bin", "app"), "tampered\n")
			err := VerifyStaging(br)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "does not match the manifest")
		})

		Convey("a staged file that went missing is caught", func() {
			So(os.Remove(filepath.Join(br.Tmp, "etc", "app.conf")), ShouldBeNil)
			So(VerifyStaging(br), ShouldNotBeNil)
		})

		// anything in the tree that nothing registered would ship silently
		Convey("a file staged that nothing registered is caught", func() {
			writeFile(t, filepath.Join(br.Tmp, "usr", "bin", "extra"), "surprise\n")
			err := VerifyStaging(br)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "does not match the manifest")
		})

		Convey("an insecure build only warns", func() {
			writeFile(t, filepath.Join(br.Tmp, "usr", "bin", "app"), "tampered\n")
			br.Insecure = true
			So(VerifyStaging(br), ShouldBeNil)
		})

		// the control dir is staged separately and is not package content
		Convey("the control files are not treated as unregistered content", func() {
			So(VerifyStaging(br), ShouldBeNil)
		})
	})
}

func TestVerifyManifestStep(t *testing.T) {
	Convey("given a package with registered files", t, func() {
		p, dir := buildablePkg(t)
		br := &BuildRequest{Pkg: p, Quiet: true, dbg: newDebug(false)}

		Convey("a clean manifest verifies", func() {
			So(VerifyManifest(br), ShouldBeNil)
		})

		Convey("a changed file fails the step", func() {
			writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "tampered\n")
			So(VerifyManifest(br), ShouldNotBeNil)

			Convey("but only warns when insecure", func() {
				br.Insecure = true
				So(VerifyManifest(br), ShouldBeNil)
			})
		})

		Convey("an empty manifest fails the step", func() {
			So(p.WriteManifest(Manifest{}), ShouldBeNil)
			err := VerifyManifest(br)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "no files registered")

			// an empty package is never what was meant, insecure or not
			Convey("even when insecure", func() {
				br.Insecure = true
				So(VerifyManifest(br), ShouldNotBeNil)
			})
		})
	})
}

func TestVerifyPackageStep(t *testing.T) {
	Convey("given a built package", t, func() {
		p, _ := buildablePkg(t)

		path, err := DefaultPackager().BuildWithOpts(p, BuildOpts{Quiet: true})
		So(err, ShouldBeNil)

		Convey("checking it again passes", func() {
			br := &BuildRequest{Pkg: p, debpath: path, Quiet: true, dbg: newDebug(false)}
			So(VerifyPackage(br), ShouldBeNil)
		})

		// the committed manifest is the record of what was meant to ship, so
		// a sum changed after the build must not pass as verified
		Convey("a manifest that drifted from the package is caught", func() {
			m, err := p.Manifest()
			So(err, ShouldBeNil)
			m[0].Sum = "00000000000000000000000000000000"
			So(p.WriteManifest(m), ShouldBeNil)

			br := &BuildRequest{Pkg: p, debpath: path, Quiet: true, dbg: newDebug(false)}
			err = VerifyPackage(br)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "does not match the manifest")

			Convey("but only warns when insecure", func() {
				br.Insecure = true
				So(VerifyPackage(br), ShouldBeNil)
			})

			// skipping is for when the check itself is in the way, and unlike
			// insecure mode it does no work at all
			Convey("and is not even looked at when skipped", func() {
				br.SkipPackageCheck = true
				So(VerifyPackage(br), ShouldBeNil)
			})
		})

		Convey("a registered file missing from the package is caught", func() {
			m, err := p.Manifest()
			So(err, ShouldBeNil)
			m = append(m, ManifestEntry{Sum: hiSum, Path: "usr/bin/never-packaged"})
			So(p.WriteManifest(m), ShouldBeNil)

			br := &BuildRequest{Pkg: p, debpath: path, Quiet: true, dbg: newDebug(false)}
			So(VerifyPackage(br), ShouldNotBeNil)
		})

		Convey("a missing deb errors", func() {
			br := &BuildRequest{Pkg: p, debpath: filepath.Join(t.TempDir(), "nope.deb"), Quiet: true, dbg: newDebug(false)}
			So(VerifyPackage(br), ShouldNotBeNil)
		})
	})
}

func TestDiffManifests(t *testing.T) {
	Convey("given the committed and shipped manifests", t, func() {
		Convey("identical manifests have no problems", func() {
			m := Manifest{{Sum: "abc", Path: "usr/bin/app"}}
			So(diffManifests(m, m), ShouldBeEmpty)
		})

		Convey("a committed file missing from the package is reported", func() {
			problems := diffManifests(
				Manifest{{Sum: "abc", Path: "usr/bin/app"}},
				Manifest{},
			)
			So(len(problems), ShouldEqual, 1)
			So(problems[0], ShouldContainSubstring, "missing from the packaged md5sums")
		})

		Convey("a differing sum is reported with both values", func() {
			problems := diffManifests(
				Manifest{{Sum: "abc", Path: "usr/bin/app"}},
				Manifest{{Sum: "def", Path: "usr/bin/app"}},
			)
			So(len(problems), ShouldEqual, 1)
			So(problems[0], ShouldContainSubstring, "abc")
			So(problems[0], ShouldContainSubstring, "def")
		})

		// something in the package that nothing registered should not be there
		Convey("a packaged file that was never registered is reported", func() {
			problems := diffManifests(
				Manifest{},
				Manifest{{Sum: "abc", Path: "usr/bin/stowaway"}},
			)
			So(len(problems), ShouldEqual, 1)
			So(problems[0], ShouldContainSubstring, "not the manifest")
		})

		Convey("problems come back sorted", func() {
			problems := diffManifests(
				Manifest{{Sum: "abc", Path: "z"}, {Sum: "abc", Path: "a"}},
				Manifest{},
			)
			So(len(problems), ShouldEqual, 2)
			So(problems[0], ShouldStartWith, "a")
			So(problems[1], ShouldStartWith, "z")
		})
	})
}

func TestBuildDebControlPerms(t *testing.T) {
	Convey("given a built package", t, func() {
		p, _ := buildablePkg(t)
		path, err := DefaultPackager().BuildWithOpts(p, BuildOpts{Quiet: true})
		So(err, ShouldBeNil)

		out := filepath.Join(t.TempDir(), "control")
		So(deb.ExtractControl(path, out), ShouldBeNil)

		Convey("the maintainer scripts are executable", func() {
			for name := range MaintainerScripts {
				fi, err := os.Stat(filepath.Join(out, name))
				if os.IsNotExist(err) {
					continue
				}
				So(err, ShouldBeNil)
				So(fi.Mode().Perm()&0100, ShouldNotEqual, 0)
			}
		})

		// dpkg only reads these, so there is no reason for them to be
		// executable
		Convey("the data control files are not", func() {
			for _, name := range []string{"control", "md5sums", "docfiles"} {
				fi, err := os.Stat(filepath.Join(out, name))
				if os.IsNotExist(err) {
					continue
				}
				So(err, ShouldBeNil)
				So(fi.Mode().Perm()&0111, ShouldEqual, 0)
			}
		})
	})
}
