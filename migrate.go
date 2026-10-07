package ian

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/penguinpowernz/go-ian/util/file"
	"github.com/penguinpowernz/go-ian/util/str"
)

// legacyIgnoreFile is the ignore file used before the manifest format, kept
// here (rather than in the deleted ignore.go) purely so migration can read it
const legacyIgnoreFile = ".ianignore"

// legacyExcludes are the patterns the old packager always excluded, regardless
// of the contents of .ianignore
var legacyExcludes = []string{
	".git", "pkg", ".gitignore", ".ianpush", ".ianignore", ".gitkeep", "./.*",
}

// LegacyIgnoreFile returns the path to the pre-manifest ignore file
func (p *Pkg) LegacyIgnoreFile() string {
	return p.Dir(legacyIgnoreFile)
}

// LegacyExcludes returns the exclude patterns the old packager would have used
// for this package: the contents of .ianignore plus the built in defaults.
func (p *Pkg) LegacyExcludes() []string {
	var patterns []string

	if data, err := ioutil.ReadFile(p.LegacyIgnoreFile()); err == nil {
		for _, l := range str.CleanStrings(str.Lines(string(data))) {
			if l == "" || strings.HasPrefix(l, "#") {
				continue
			}
			patterns = append(patterns, l)
		}
	}

	return append(patterns, legacyExcludes...)
}

// legacyExcluded reports whether a repo-relative path would have been excluded
// by the old rsync based packager, given its exclude patterns.  rsync matches a
// pattern containing no slash against each path component, and one containing a
// slash against the path from the package root, which is what is reproduced
// here well enough for a one-off migration.
func legacyExcluded(rel string, patterns []string) bool {
	components := strings.Split(rel, string(filepath.Separator))

	for _, pat := range patterns {
		if pat == "" {
			continue
		}

		// rsync anchors a leading / or ./ to the transfer root
		anchored := strings.TrimPrefix(strings.TrimPrefix(pat, "./"), "/")

		if strings.Contains(anchored, "/") {
			if ok, _ := filepath.Match(anchored, rel); ok {
				return true
			}
			// a directory pattern excludes everything beneath it
			if strings.HasPrefix(rel, strings.TrimSuffix(anchored, "/")+"/") {
				return true
			}
			continue
		}

		// an unanchored pattern matches any component of the path, which is how
		// ".git" excludes the whole directory wherever it appears
		for _, c := range components {
			if ok, _ := filepath.Match(anchored, c); ok {
				return true
			}
		}
	}

	return false
}

// MigrationPlan describes what migrating a legacy package to the manifest
// format would change, so it can be shown before anything is written.
type MigrationPlan struct {
	Files    []string // repo-relative files to register in the manifest
	Docs     []string // bare root files to register as doc files
	Excluded []string // files skipped because the old excludes covered them

	// Legacy is the ignore file that was consulted, empty when there was none.
	// Its presence is what makes a package "legacy".
	Legacy string

	// AlreadyRegistered is true when the package already has manifest entries,
	// meaning migration has probably already happened.
	AlreadyRegistered bool
}

// Empty reports whether the plan would register nothing at all
func (mp MigrationPlan) Empty() bool {
	return len(mp.Files) == 0 && len(mp.Docs) == 0
}

// PlanMigration works out how to move a pre-manifest package to the manifest
// format.  Every file the old packager would have included is registered in the
// manifest, except bare files in the repo root, which the old packager swept
// into usr/share/doc/<package> and which therefore become doc files instead.
//
// Nothing is written: the plan is returned for the caller to show and apply.
func (p *Pkg) PlanMigration() (MigrationPlan, error) {
	var mp MigrationPlan

	if file.Exists(p.LegacyIgnoreFile()) {
		mp.Legacy = p.LegacyIgnoreFile()
	}

	m, err := p.Manifest()
	if err != nil {
		return mp, err
	}
	mp.AlreadyRegistered = len(m) > 0

	patterns := p.LegacyExcludes()
	docDir := p.DocDir()

	err = filepath.Walk(p.Dir(), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := p.relToDir(path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		if info.IsDir() {
			// the control dir and the build output are never package contents,
			// and the doc dir is managed through docfiles
			if rel == "DEBIAN" || rel == docDir {
				return filepath.SkipDir
			}
			if legacyExcluded(rel, patterns) {
				mp.Excluded = append(mp.Excluded, rel+"/")
				return filepath.SkipDir
			}
			return nil
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		if legacyExcluded(rel, patterns) {
			mp.Excluded = append(mp.Excluded, rel)
			return nil
		}

		// a bare file in the repo root is what the old CleanRoot step moved
		// into the doc dir, so preserve that placement
		if !strings.Contains(rel, string(filepath.Separator)) {
			mp.Docs = append(mp.Docs, rel)
			return nil
		}

		mp.Files = append(mp.Files, rel)
		return nil
	})
	if err != nil {
		return mp, err
	}

	sort.Strings(mp.Files)
	sort.Strings(mp.Docs)
	sort.Strings(mp.Excluded)

	return mp, nil
}

// ApplyMigration registers everything in the plan, writing the manifest and the
// docfiles list.  The legacy ignore file is left in place; removing it is left
// to the caller so that the migration can be reviewed against it first.
func (p *Pkg) ApplyMigration(mp MigrationPlan) error {
	for _, f := range mp.Files {
		if err := p.AddFile(f); err != nil {
			return fmt.Errorf("failed to register %s: %s", f, err)
		}
	}

	for _, d := range mp.Docs {
		if err := p.AddDocFile(d); err != nil {
			return fmt.Errorf("failed to register doc file %s: %s", d, err)
		}
	}

	return nil
}
