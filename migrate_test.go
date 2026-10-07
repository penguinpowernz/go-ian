package ian

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLegacyExcluded(t *testing.T) {
	Convey("given the old packager's exclude patterns", t, func() {
		Convey("an unanchored pattern matches any path component", func() {
			So(legacyExcluded(".git", []string{".git"}), ShouldBeTrue)
			So(legacyExcluded("usr/.git/config", []string{".git"}), ShouldBeTrue)
			So(legacyExcluded("usr/bin/app", []string{".git"}), ShouldBeFalse)
		})

		Convey("a pattern with a slash is anchored to the package root", func() {
			So(legacyExcluded("usr/bin", []string{"usr/bin"}), ShouldBeTrue)
			So(legacyExcluded("etc/usr/bin", []string{"usr/bin"}), ShouldBeFalse)
		})

		Convey("an anchored directory pattern excludes everything beneath it", func() {
			So(legacyExcluded("usr/bin/app", []string{"usr/bin"}), ShouldBeTrue)
			So(legacyExcluded("usr/bin/deep/app", []string{"usr/bin/"}), ShouldBeTrue)
		})

		Convey("a leading / or ./ is stripped before matching", func() {
			So(legacyExcluded("usr/bin", []string{"/usr/bin"}), ShouldBeTrue)
			So(legacyExcluded("usr/bin", []string{"./usr/bin"}), ShouldBeTrue)
		})

		Convey("globs are honoured", func() {
			So(legacyExcluded("build/out.o", []string{"*.o"}), ShouldBeTrue)
			So(legacyExcluded("build/out.c", []string{"*.o"}), ShouldBeFalse)
		})

		// the default ./.* pattern is what hid dotfiles from the old packager
		Convey("the built in defaults hide dotfiles and the build output", func() {
			pats := legacyExcludes
			So(legacyExcluded(".gitignore", pats), ShouldBeTrue)
			So(legacyExcluded(".ianpush", pats), ShouldBeTrue)
			So(legacyExcluded("pkg/old.deb", pats), ShouldBeTrue)
			So(legacyExcluded("usr/bin/app", pats), ShouldBeFalse)
		})

		Convey("an empty pattern matches nothing", func() {
			So(legacyExcluded("usr/bin/app", []string{""}), ShouldBeFalse)
		})
	})
}

func TestLegacyExcludes(t *testing.T) {
	Convey("given a package with no ignore file", t, func() {
		p, _ := newPkg(t)

		Convey("only the built in defaults apply", func() {
			So(p.LegacyExcludes(), ShouldResemble, legacyExcludes)
		})
	})

	Convey("given a package with an ignore file", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, ".ianignore"), "# a comment\n\nbuild\n  spaced  \n")

		Convey("its patterns come first, then the defaults", func() {
			pats := p.LegacyExcludes()
			So(pats[0], ShouldEqual, "build")

			// comments and blank lines are not patterns, and entries are
			// trimmed, so an indented line still excludes what it names
			So(pats[1], ShouldEqual, "spaced")
			So(pats, ShouldNotContain, "# a comment")
			So(pats, ShouldNotContain, "")
			So(pats, ShouldContain, ".git")
		})

		Convey("the ignore file path is the one it read", func() {
			So(p.LegacyIgnoreFile(), ShouldEqual, filepath.Join(dir, ".ianignore"))
		})
	})
}

func TestMigrationPlanEmpty(t *testing.T) {
	Convey("a plan registering nothing is empty", t, func() {
		So(MigrationPlan{}.Empty(), ShouldBeTrue)
		So(MigrationPlan{Excluded: []string{"pkg/"}}.Empty(), ShouldBeTrue)
		So(MigrationPlan{Files: []string{"usr/bin/app"}}.Empty(), ShouldBeFalse)
		So(MigrationPlan{Docs: []string{"README.md"}}.Empty(), ShouldBeFalse)
	})
}

func TestPlanMigration(t *testing.T) {
	Convey("given a legacy package", t, func() {
		p, dir := newPkg(t)

		writeFile(t, filepath.Join(dir, ".ianignore"), "build\n")
		writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "hi\n")
		writeFile(t, filepath.Join(dir, "etc", "app.conf"), "hi\n")
		writeFile(t, filepath.Join(dir, "README.md"), "hi\n")
		writeFile(t, filepath.Join(dir, "build", "junk.o"), "hi\n")
		writeFile(t, filepath.Join(dir, "pkg", "old.deb"), "hi\n")
		writeFile(t, filepath.Join(dir, ".gitignore"), "hi\n")

		mp, err := p.PlanMigration()
		So(err, ShouldBeNil)

		Convey("the ignore file marks it as legacy", func() {
			So(mp.Legacy, ShouldEqual, p.LegacyIgnoreFile())
		})

		Convey("files in subdirectories become manifest entries", func() {
			So(mp.Files, ShouldResemble, []string{"etc/app.conf", "usr/bin/app"})
		})

		// the old packager swept bare root files into the doc dir, so that
		// placement has to be preserved
		Convey("bare root files become doc files", func() {
			So(mp.Docs, ShouldResemble, []string{"README.md"})
		})

		Convey("what the old excludes covered is skipped and reported", func() {
			So(mp.Excluded, ShouldContain, "build/")
			So(mp.Excluded, ShouldContain, "pkg/")
			So(mp.Excluded, ShouldContain, ".gitignore")

			// an excluded directory is not descended into
			So(mp.Excluded, ShouldNotContain, "build/junk.o")
			So(mp.Files, ShouldNotContain, "build/junk.o")
		})

		Convey("the control dir is never package content", func() {
			for _, f := range mp.Files {
				So(f, ShouldNotStartWith, "DEBIAN")
			}
			So(mp.Docs, ShouldNotContain, "control")
		})

		Convey("nothing has been written yet", func() {
			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m, ShouldBeEmpty)
		})

		Convey("it is not reported as already migrated", func() {
			So(mp.AlreadyRegistered, ShouldBeFalse)
		})
	})

	Convey("given a package with no ignore file", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "hi\n")

		mp, err := p.PlanMigration()
		So(err, ShouldBeNil)

		Convey("it is not marked legacy, but still plans the files", func() {
			So(mp.Legacy, ShouldEqual, "")
			So(mp.Files, ShouldResemble, []string{"usr/bin/app"})
		})
	})

	Convey("given a package that already has manifest entries", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "hi\n")
		So(p.AddFile("usr/bin/app"), ShouldBeNil)

		mp, err := p.PlanMigration()
		So(err, ShouldBeNil)

		Convey("it says so, so the caller can warn before migrating again", func() {
			So(mp.AlreadyRegistered, ShouldBeTrue)
		})
	})

	Convey("given a package with files already in the doc dir", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, p.DocDir(), "guide.md"), "hi\n")
		writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "hi\n")

		mp, err := p.PlanMigration()
		So(err, ShouldBeNil)

		// the doc dir is managed through docfiles, so walking into it would
		// register its contents as their own source
		Convey("the doc dir is left out of the plan", func() {
			So(mp.Files, ShouldResemble, []string{"usr/bin/app"})
			for _, f := range mp.Files {
				So(f, ShouldNotStartWith, p.DocDir())
			}
		})
	})

	Convey("given a package with a symlink in it", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "hi\n")

		if err := os.Symlink(filepath.Join(dir, "usr", "bin", "app"), filepath.Join(dir, "usr", "bin", "link")); err == nil {
			mp, err := p.PlanMigration()
			So(err, ShouldBeNil)

			Convey("only regular files are planned", func() {
				So(mp.Files, ShouldResemble, []string{"usr/bin/app"})
			})
		}
	})
}

func TestApplyMigration(t *testing.T) {
	Convey("given a legacy package and its migration plan", t, func() {
		p, dir := newPkg(t)

		writeFile(t, filepath.Join(dir, ".ianignore"), "build\n")
		writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "hi\n")
		writeFile(t, filepath.Join(dir, "README.md"), "hi\n")
		writeFile(t, filepath.Join(dir, "build", "junk.o"), "hi\n")

		mp, err := p.PlanMigration()
		So(err, ShouldBeNil)
		So(p.ApplyMigration(mp), ShouldBeNil)

		Convey("the ordinary files are registered at their own paths", func() {
			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m.Paths(), ShouldContain, "usr/bin/app")
		})

		Convey("the root files are registered as doc files", func() {
			d, err := p.DocFiles()
			So(err, ShouldBeNil)
			So(d, ShouldResemble, DocFiles{"README.md"})

			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m.Paths(), ShouldContain, p.DocDest("README.md"))
		})

		Convey("the excluded files are not registered", func() {
			m, err := p.Manifest()
			So(err, ShouldBeNil)
			So(m.Paths(), ShouldNotContain, "build/junk.o")
		})

		Convey("the migrated package verifies cleanly", func() {
			problems, err := p.Verify(false)
			So(err, ShouldBeNil)
			So(problems, ShouldBeEmpty)
		})

		// removing it is left to the caller so the migration can be reviewed
		// against what it was based on
		Convey("the legacy ignore file is left in place", func() {
			So(fexists(p.LegacyIgnoreFile()), ShouldBeTrue)
		})
	})

	Convey("given a plan naming a file that has since gone", t, func() {
		p, dir := newPkg(t)
		writeFile(t, filepath.Join(dir, "usr", "bin", "app"), "hi\n")

		mp, err := p.PlanMigration()
		So(err, ShouldBeNil)
		So(os.Remove(filepath.Join(dir, "usr", "bin", "app")), ShouldBeNil)

		Convey("applying it errors, naming the file", func() {
			err := p.ApplyMigration(mp)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "usr/bin/app")
		})
	})
}
