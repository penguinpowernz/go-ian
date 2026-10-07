package ian

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/penguinpowernz/go-ian/util/file"
	"github.com/penguinpowernz/go-ian/util/str"
)

// ManifestEntry is a single line in the DEBIAN/md5sums manifest: an MD5
// sum and the repo-relative path of the file it belongs to.  Paths use the
// standard Debian md5sums form with no leading slash (e.g. "bin/thing").
type ManifestEntry struct {
	Sum  string
	Path string
}

// Manifest is the list of files that make up a package, read from the
// DEBIAN/md5sums file.  It is the source of truth for what gets included
// in the package: nothing is packaged unless it appears here.
type Manifest []ManifestEntry

// ManifestFile returns the path to the package's md5sums manifest
func (p *Pkg) ManifestFile() string {
	return p.CtrlDir("md5sums")
}

// ReadManifest parses a DEBIAN/md5sums file at the given path.  Each line is
// of the form "<md5>  <path>".  Blank lines are skipped and malformed lines
// result in an error.
func ReadManifest(path string) (Manifest, error) {
	data, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		// a package with no manifest yet simply has no files registered; the
		// commands that need files present say so themselves
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var m Manifest
	for i, line := range str.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("malformed md5sums line %d: %q", i+1, line)
		}

		m = append(m, ManifestEntry{Sum: fields[0], Path: fields[1]})
	}

	return m, nil
}

// Manifest reads the package's DEBIAN/md5sums file into a Manifest
func (p *Pkg) Manifest() (Manifest, error) {
	return ReadManifest(p.ManifestFile())
}

// Paths returns the repo-relative paths of all files in the manifest
func (m Manifest) Paths() []string {
	paths := make([]string, 0, len(m))
	for _, e := range m {
		paths = append(paths, e.Path)
	}
	return paths
}

// Write writes the manifest in standard Debian md5sums format ("<md5>  <path>"),
// sorted by path.  The output is byte-compatible with what dpkg/debsums expect.
func (m Manifest) Write(w io.Writer) (int, error) {
	sorted := make(Manifest, len(m))
	copy(sorted, m)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	var data []byte
	for _, e := range sorted {
		data = append(data, []byte(fmt.Sprintf("%s  %s\n", e.Sum, e.Path))...)
	}

	return w.Write(data)
}

// Sum returns the MD5 sum of the file at the given path, formatted the same
// way (lowercase hex) as the sums stored in the manifest.
func Sum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// FileState describes how a file registered in the manifest compares to the
// copy currently in the repo.
type FileState int

const (
	// StateOK means the file is present and its sum matches the manifest
	StateOK FileState = iota
	// StateModified means the file is present but its sum has changed
	StateModified
	// StateMissing means the file is in the manifest but not in the repo
	StateMissing
	// StateError means the file could not be read to compute its sum
	StateError
)

// FileStatus is the result of checking one manifest entry against the repo
type FileStatus struct {
	Path   string // the path the file takes inside the package
	Source string // the repo file it is staged from, differs for doc files
	State  FileState
	Want   string // the sum recorded in the manifest
	Got    string // the sum computed from the repo, empty when unavailable
	Err    error  // set when State is StateError
}

// IsDoc reports whether this entry is staged from a different path than it
// takes in the package, which is what makes it a doc file
func (s FileStatus) IsDoc() bool {
	return s.Source != "" && s.Source != s.Path
}

// Problem renders the status as a human readable problem description, or
// returns an empty string when the file is OK.
func (s FileStatus) Problem() string {
	// report the repo file, since that is the one to go and look at, noting the
	// package path too when the two differ
	what := s.Source
	if what == "" {
		what = s.Path
	}
	if s.IsDoc() {
		what = fmt.Sprintf("%s (installs as %s)", s.Source, s.Path)
	}

	switch s.State {
	case StateMissing:
		return fmt.Sprintf("%s: missing from repo", what)
	case StateError:
		return fmt.Sprintf("%s: %s", what, s.Err)
	case StateModified:
		return fmt.Sprintf("%s: md5 mismatch (want %s, got %s)", what, s.Want, s.Got)
	}
	return ""
}

// Status checks every file in the manifest against its recorded sum and returns
// the state of each one, in manifest order.  Unlike Verify it never fails on a
// mismatch, so callers can report drift without aborting.
func (p *Pkg) Status() ([]FileStatus, error) {
	m, err := p.Manifest()
	if err != nil {
		return nil, err
	}

	// doc files are staged from somewhere other than their path in the package,
	// so their sums must be checked against the source file in the repo
	docSrcs, err := p.DocSources()
	if err != nil {
		return nil, err
	}

	statuses := make([]FileStatus, 0, len(m))
	for _, e := range m {
		src := p.SourceFor(e.Path, docSrcs)
		st := FileStatus{Path: e.Path, Source: src, Want: e.Sum}
		path := p.Dir(src)

		switch {
		case !file.Exists(path):
			st.State = StateMissing
		default:
			sum, err := Sum(path)
			switch {
			case err != nil:
				st.State = StateError
				st.Err = err
			case sum != e.Sum:
				st.State = StateModified
				st.Got = sum
			default:
				st.State = StateOK
				st.Got = sum
			}
		}

		statuses = append(statuses, st)
	}

	return statuses, nil
}

// Verify checks every file in the manifest against its recorded sum, computing
// the sum of the corresponding repo file.  It returns a list of human readable
// problems (mismatches and missing files).  When insecure is false any problem
// also results in an error; when insecure is true the problems are returned
// without an error so the caller can warn and continue.
func (p *Pkg) Verify(insecure bool) ([]string, error) {
	statuses, err := p.Status()
	if err != nil {
		return nil, err
	}

	var problems []string
	for _, st := range statuses {
		if msg := st.Problem(); msg != "" {
			problems = append(problems, msg)
		}
	}

	if len(problems) > 0 && !insecure {
		return problems, fmt.Errorf("md5sum verification failed for %d file(s)", len(problems))
	}

	return problems, nil
}

// manifestPath normalizes a user supplied path into the repo-relative form used
// in the manifest, rejecting control files.  They are staged separately and are
// not part of the package's file system, so they must never appear in the
// manifest - and md5sums could never hold a stable sum of itself.
func manifestPath(relpath string) (string, error) {
	relpath = strings.TrimPrefix(relpath, "./")
	relpath = strings.TrimPrefix(relpath, "/")

	if relpath == "DEBIAN" || strings.HasPrefix(relpath, "DEBIAN/") {
		return "", fmt.Errorf("%s: control files cannot be registered in the manifest", relpath)
	}

	return relpath, nil
}

// WriteManifest writes the manifest to the package's md5sums file, replacing
// any existing contents.
func (p *Pkg) WriteManifest(m Manifest) error {
	f, err := os.OpenFile(p.ManifestFile(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = m.Write(f)
	return err
}

// ExpandFiles resolves the given paths into a list of repo-relative file paths
// suitable for registering in the manifest.  Directories are walked recursively
// so that `ian add .` or `ian add usr` registers everything beneath them, while
// plain files are passed through.  Paths that can never belong in the package
// are skipped rather than erroring, so that shell globs and `.` behave sensibly:
// the DEBIAN control dir, the pkg output dir, ian's own dotfiles and VCS
// metadata.  Symlinks are not followed.
func (p *Pkg) ExpandFiles(paths []string) ([]string, error) {
	var found []string
	seen := map[string]bool{}

	for _, arg := range paths {
		// make the argument relative to the package dir so that walking
		// produces manifest-shaped paths regardless of how it was given
		rel, err := p.relToDir(arg)
		if err != nil {
			return nil, err
		}

		err = filepath.Walk(p.Dir(rel), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			r, err := p.relToDir(path)
			if err != nil {
				return err
			}

			if info.IsDir() {
				// the doc dir holds files placed there by `ian doc`, which are
				// registered at their destination rather than walked
				if skipDir(r) || r == p.DocDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// only regular files can be summed and shipped
			if !info.Mode().IsRegular() || skipFile(r) || seen[r] {
				return nil
			}

			seen[r] = true
			found = append(found, r)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Strings(found)
	return found, nil
}

// relToDir expresses the given path relative to the package dir, rejecting
// anything that resolves outside of it.  Relative paths are taken as being
// relative to the package dir itself rather than the working directory, so that
// `ian -d some/pkg add .` means that package's root and not the caller's cwd.
func (p *Pkg) relToDir(path string) (string, error) {
	root, err := filepath.Abs(p.dir)
	if err != nil {
		return "", err
	}

	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}

	abs, err = filepath.Abs(abs)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s: outside of the package directory", path)
	}

	return rel, nil
}

// skipDir reports whether a directory should be skipped entirely when expanding
func skipDir(rel string) bool {
	switch rel {
	case ".":
		return false
	case "DEBIAN", "pkg", ".git":
		return true
	}
	return false
}

// skipFile reports whether a file should be left out when expanding a directory
func skipFile(rel string) bool {
	switch filepath.Base(rel) {
	case ".gitignore", ".gitkeep", ".ianpush", ".ianignore":
		return true
	}
	return false
}

// AddFile computes the MD5 sum of the given repo-relative file and upserts its
// entry into the manifest, writing the updated manifest back to disk.
func (p *Pkg) AddFile(relpath string) error {
	relpath, err := manifestPath(relpath)
	if err != nil {
		return err
	}

	sum, err := Sum(p.Dir(relpath))
	if err != nil {
		return err
	}

	return p.addEntry(relpath, sum)
}

// addEntry upserts a single path and sum into the manifest and writes it back.
// The path is the one the file takes inside the package, which for doc files
// differs from the repo path they are copied from.
func (p *Pkg) addEntry(path, sum string) error {
	m, err := p.Manifest()
	if err != nil {
		return err
	}

	found := false
	for i, e := range m {
		if e.Path == path {
			m[i].Sum = sum
			found = true
			break
		}
	}
	if !found {
		m = append(m, ManifestEntry{Sum: sum, Path: path})
	}

	return p.WriteManifest(m)
}

// RemoveFiles drops the given paths' entries from the manifest, writing the
// updated manifest back to disk.  The files themselves are left alone:
// unregistering only means they are no longer included in the package.
//
// A path matching a registered file removes that entry; a path naming a
// directory (or "." for the whole package) removes every entry beneath it.
// Matching is done against the manifest rather than the disk, so entries whose
// files have already been deleted can still be unregistered.  It returns the
// removed paths, and an error if any argument matched nothing.
func (p *Pkg) RemoveFiles(paths []string) ([]string, error) {
	m, err := p.Manifest()
	if err != nil {
		return nil, err
	}

	drop := map[string]bool{}
	for _, arg := range paths {
		rel, err := p.relToDir(arg)
		if err != nil {
			return nil, err
		}

		matched := false
		for _, e := range m {
			if e.Path == rel || rel == "." || strings.HasPrefix(e.Path, rel+"/") {
				drop[e.Path] = true
				matched = true
			}
		}

		if !matched {
			return nil, fmt.Errorf("%s: not registered in the manifest", arg)
		}
	}

	kept := make(Manifest, 0, len(m))
	var removed []string
	for _, e := range m {
		if drop[e.Path] {
			removed = append(removed, e.Path)
			continue
		}
		kept = append(kept, e)
	}

	sort.Strings(removed)
	return removed, p.WriteManifest(kept)
}
