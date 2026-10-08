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

// ManifestName is the base name of the architecture independent manifest.
// An arch-qualified manifest is this plus ".<arch>".
const ManifestName = "md5sums"

// ArchAll is the control file architecture for a package whose contents are
// the same on every architecture, and so has no per-arch manifests
const ArchAll = "all"

// manifestFor returns the base name of the manifest for the given
// architecture.  An unset arch and "all" both name the plain manifest, since
// neither describes contents that vary between architectures.
func manifestFor(arch string) string {
	if arch == "" || arch == ArchAll {
		return ManifestName
	}
	return ManifestName + "." + arch
}

// IsManifestFile reports whether a control file base name is a manifest, either
// the plain one or any arch-qualified sibling.  Staging uses this to leave the
// siblings out of the package: only the manifest for the arch being built is
// shipped, and it is shipped as plain "md5sums" so dpkg and debsums find it.
func IsManifestFile(name string) bool {
	return name == ManifestName || strings.HasPrefix(name, ManifestName+".")
}

// ManifestFile returns the path to the manifest this package builds from, which
// is the one reading falls back through.  See WriteManifestFile for the
// asymmetry between reading and writing.
//
// What goes into a package is per architecture: the same package path (say
// usr/bin/thing) holds different bytes in the amd64 build than in the arm64
// one, so a single md5sums file can only ever vouch for whichever arch was
// built last.  The sums are therefore kept per arch in DEBIAN/md5sums.<arch>,
// which lets every architecture's sums be committed side by side and each
// build verify strictly against its own.
//
// Packages that predate this (and any package built for a single arch) keep
// working: when this arch has no manifest of its own, the plain DEBIAN/md5sums
// is read instead.
func (p *Pkg) ManifestFile() string {
	name := manifestFor(p.ctrl.Arch)

	// "all" (and an unset arch) means the contents do not vary by
	// architecture, so the plain manifest is already the right name for them
	// and there is nothing to fall back to
	if name == ManifestName {
		return p.CtrlDir(ManifestName)
	}

	qualified := p.CtrlDir(name)
	if file.Exists(qualified) {
		return qualified
	}

	// fall back to the plain manifest when this arch has none of its own, so a
	// package carrying only the old single manifest keeps building unchanged
	if file.Exists(p.CtrlDir(ManifestName)) {
		return p.CtrlDir(ManifestName)
	}

	return qualified
}

// WriteManifestFile returns the path that registering a file writes to, which
// is always the manifest for the package's own architecture.
//
// Writing deliberately does not follow ManifestFile's fallback.  If it did,
// every arch would keep writing into the plain manifest that `ian init` leaves
// behind, each build overwriting the last arch's sums, and the package could
// never grow a second manifest.  Writing to the arch-qualified name instead
// means the first `ian add` after `ian set -a` splits that arch off into its
// own manifest, seeded from whatever the fallback was reading.
func (p *Pkg) WriteManifestFile() string {
	return p.CtrlDir(manifestFor(p.ctrl.Arch))
}

// ManifestFiles returns the paths of every manifest in the control dir, the
// plain one and all arch-qualified siblings, sorted.
func (p *Pkg) ManifestFiles() []string {
	var found []string
	for _, fpath := range p.CtrlFiles() {
		if IsManifestFile(filepath.Base(fpath)) {
			found = append(found, fpath)
		}
	}
	sort.Strings(found)
	return found
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

		// the manifest is committed to git and editable by hand, so the paths
		// coming out of it get the same scrutiny as the ones going in
		path, err := manifestPath(fields[1])
		if err != nil {
			return nil, fmt.Errorf("md5sums line %d: %s", i+1, err)
		}

		m = append(m, ManifestEntry{Sum: fields[0], Path: path})
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

// manifestPath normalizes a path into the repo-relative form used in the
// manifest, rejecting anything that could not legitimately belong there.
//
// Control files are rejected because they are staged separately and are not
// part of the package's file system, so they must never appear in the manifest
// - and md5sums could never hold a stable sum of itself.
//
// Paths that escape the package directory are rejected because every path in
// the manifest and the docfiles list is joined against both the repo root (to
// read the file) and the staging root (to write it).  A path containing ".."
// would read a file from outside the repo or write one outside the staging
// dir, so a hand edited DEBIAN/md5sums could otherwise pull an arbitrary file
// into the package or drop a file anywhere the build user can write.  This is
// enforced on the way out of the control files as well as on the way in, since
// they are committed to git and editable by hand.
func manifestPath(relpath string) (string, error) {
	orig := relpath

	if filepath.IsAbs(relpath) {
		return "", fmt.Errorf("%s: must be relative to the package directory", orig)
	}

	relpath = strings.TrimPrefix(relpath, "./")

	// Clean resolves any interior ".." and strips trailing slashes, so a
	// leading ".." afterwards is the only way left to point outside the repo
	relpath = filepath.Clean(relpath)

	if relpath == ".." || strings.HasPrefix(relpath, "../") {
		return "", fmt.Errorf("%s: outside of the package directory", orig)
	}

	if relpath == "." {
		return "", fmt.Errorf("%s: not a file", orig)
	}

	if relpath == "DEBIAN" || strings.HasPrefix(relpath, "DEBIAN/") {
		return "", fmt.Errorf("%s: control files cannot be registered in the manifest", orig)
	}

	return relpath, nil
}

// confine joins rel onto root and checks the result is still inside root.
// ReadManifest and ReadDocFiles already reject escaping paths, so this is the
// backstop for the filesystem calls themselves: staging reads and writes are
// the point where a bad path would do damage, and they should not depend on
// every caller having validated it first.
func confine(root, rel string) (string, error) {
	path := filepath.Join(root, rel)

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	r, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", err
	}

	if r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s: resolves outside of %s", rel, root)
	}

	return path, nil
}

// WriteManifest writes the manifest to the md5sums file for the package's
// architecture, replacing any existing contents.  Since the callers read
// through ManifestFile's fallback and write here, registering a file against an
// arch that has no manifest yet copies the fallback's entries into a new
// arch-qualified manifest along with the new one.
func (p *Pkg) WriteManifest(m Manifest) error {
	return writeManifestFile(p.WriteManifestFile(), m)
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
//
// A file already registered as a doc file is recorded at its destination under
// the doc dir, so adding it again re-sums the entry it already has instead of
// registering the repo path a second time.
func (p *Pkg) AddFile(relpath string) error {
	relpath, err := manifestPath(relpath)
	if err != nil {
		return err
	}

	sum, err := Sum(p.Dir(relpath))
	if err != nil {
		return err
	}

	path, err := p.ManifestPathFor(relpath)
	if err != nil {
		return err
	}

	return p.addEntry(path, sum)
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

// UpdateFiles re-sums every registered file whose contents no longer match the
// manifest, writing the updated manifest back to disk.  This is the bulk form
// of re-running `ian add` over each file that `ian status` reports as modified,
// so that a round of edits can be re-registered in one go.
//
// Only modified entries are touched.  Entries that are missing from the repo or
// cannot be read are left alone and returned as problems, since re-summing is
// not what they need: a missing file has to be either restored or unregistered
// with `ian rm`, and silently dropping it would quietly shrink the package.
//
// It returns the paths that were re-summed, in sorted order, and the problems
// for the entries it could not update.
func (p *Pkg) UpdateFiles() ([]string, []string, error) {
	statuses, err := p.Status()
	if err != nil {
		return nil, nil, err
	}

	m, err := p.Manifest()
	if err != nil {
		return nil, nil, err
	}

	// index the manifest by path so each drifted entry can be updated in place,
	// which keeps the sums of everything else exactly as they were
	at := make(map[string]int, len(m))
	for i, e := range m {
		at[e.Path] = i
	}

	var updated, problems []string
	for _, st := range statuses {
		if st.State != StateModified {
			if msg := st.Problem(); msg != "" {
				problems = append(problems, msg)
			}
			continue
		}

		i, ok := at[st.Path]
		if !ok {
			continue
		}

		// Status already computed the sum of the file on disk, so take it from
		// there rather than reading every drifted file a second time
		m[i].Sum = st.Got
		updated = append(updated, st.Path)
	}

	if len(updated) == 0 {
		return nil, problems, nil
	}

	sort.Strings(updated)
	return updated, problems, p.WriteManifest(m)
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
//
// Unregistering a doc file's destination also drops it from DEBIAN/docfiles, so
// that the list never names a file the manifest no longer carries.
func (p *Pkg) RemoveFiles(paths []string) ([]string, error) {
	m, err := p.Manifest()
	if err != nil {
		return nil, err
	}

	drop, err := p.resolveRemovals(paths, m)
	if err != nil {
		return nil, err
	}

	removed, kept := partition(m, drop)
	if err := p.WriteManifest(kept); err != nil {
		return nil, err
	}

	return removed, p.pruneDocFiles(drop)
}

// RemoveFilesAllArches drops the given paths' entries from every manifest in
// the control dir rather than only the one for the package's own architecture.
//
// This is the counterpart to AddFileAllArches: a file registered across every
// arch takes as many commands to unregister again, one per arch, with the
// manifests silently disagreeing in between.  The paths to drop are resolved
// against each manifest in turn, so a directory argument removes whatever that
// manifest happens to hold beneath it even where the arches differ.
//
// An argument is an error only when no manifest at all records it, since a file
// genuinely present in some arches and not others is exactly what this is for.
// It returns the removed paths, sorted and deduplicated across the manifests,
// and the base names of the manifests it wrote.
func (p *Pkg) RemoveFilesAllArches(paths []string) ([]string, []string, error) {
	targets := p.ManifestFiles()
	if len(targets) == 0 {
		targets = []string{p.ManifestFile()}
	}

	// resolve against every manifest before writing any of them, so that an
	// argument matching nothing anywhere leaves the manifests untouched rather
	// than editing the ones that came before it in the list
	drops := make([]map[string]bool, len(targets))
	manifests := make([]Manifest, len(targets))

	// an argument is only unregistered if no manifest matched it, so track
	// which ones have matched somewhere and judge them once all are read
	everMatched := map[string]bool{}

	for i, mf := range targets {
		m, err := ReadManifest(mf)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %s", filepath.Base(mf), err)
		}
		manifests[i] = m

		drop, matched, err := p.matchRemovals(paths, m)
		if err != nil {
			return nil, nil, err
		}

		drops[i] = drop
		for _, arg := range matched {
			everMatched[arg] = true
		}
	}

	for _, arg := range paths {
		if !everMatched[arg] {
			return nil, nil, fmt.Errorf("%s: not registered in any manifest", arg)
		}
	}

	var removed, written []string
	for i, mf := range targets {
		r, kept := partition(manifests[i], drops[i])
		removed = append(removed, r...)

		if err := writeManifestFile(mf, kept); err != nil {
			return nil, nil, err
		}
		written = append(written, filepath.Base(mf))
	}

	sort.Strings(written)

	if err := p.pruneDocFiles(allDropped(drops)); err != nil {
		return nil, nil, err
	}

	return dedupe(removed), written, nil
}

// allDropped unions the per-manifest removals into one set of dropped paths
func allDropped(drops []map[string]bool) map[string]bool {
	all := map[string]bool{}
	for _, drop := range drops {
		for path := range drop {
			all[path] = true
		}
	}
	return all
}

// pruneDocFiles drops from DEBIAN/docfiles every entry whose destination is
// among the manifest paths just removed, keeping the list and the manifest in
// agreement about which files the package carries.  A doc file left in the list
// with no manifest entry would be staged into the package unregistered, and
// would reappear in the manifest the next time anything re-added it.
func (p *Pkg) pruneDocFiles(dropped map[string]bool) error {
	if len(dropped) == 0 {
		return nil
	}

	d, err := p.DocFiles()
	if err != nil {
		return err
	}

	kept := make(DocFiles, 0, len(d))
	for _, src := range d {
		if dropped[p.DocDest(src)] {
			continue
		}
		kept = append(kept, src)
	}

	if len(kept) == len(d) {
		return nil
	}

	return p.WriteDocFiles(kept)
}

// resolveRemovals works out which of the manifest's entries the given arguments
// name, erroring on an argument that matches none of them.
//
// A path registered as a doc file is matched by the entry it installs to under
// the doc dir, so that `ian rm docs/guide.md` unregisters the same entry that
// `ian add docs/guide.md` wrote, rather than reporting the repo path as
// unregistered because the manifest records only the destination.
func (p *Pkg) resolveRemovals(paths []string, m Manifest) (map[string]bool, error) {
	drop, matched, err := p.matchRemovals(paths, m)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	for _, arg := range matched {
		seen[arg] = true
	}

	for _, arg := range paths {
		if !seen[arg] {
			return nil, fmt.Errorf("%s: not registered in the manifest", arg)
		}
	}

	return drop, nil
}

// matchRemovals maps the given arguments onto the manifest entries they name,
// returning the entries to drop and the arguments that matched at least one.
//
// A path registered as a doc file is matched by the entry it installs to under
// the doc dir, so that removal finds the same entry that `ian add` wrote rather
// than missing it because the manifest records only the destination.
func (p *Pkg) matchRemovals(paths []string, m Manifest) (map[string]bool, []string, error) {
	docSrcs, err := p.DocSources()
	if err != nil {
		return nil, nil, err
	}

	drop := map[string]bool{}
	var matched []string

	for _, arg := range paths {
		rel, err := p.relToDir(arg)
		if err != nil {
			return nil, nil, err
		}

		// a doc file is registered under the doc dir, so look for the argument
		// there as well as at its own path
		want := map[string]bool{rel: true}
		for dst, src := range docSrcs {
			if src == rel || rel == "." || strings.HasPrefix(src, rel+"/") {
				want[dst] = true
			}
		}

		hit := false
		for _, e := range m {
			if want[e.Path] || rel == "." || strings.HasPrefix(e.Path, rel+"/") {
				drop[e.Path] = true
				hit = true
			}
		}

		if hit {
			matched = append(matched, arg)
		}
	}

	return drop, matched, nil
}

// partition splits a manifest into the entries to drop and the ones to keep
func partition(m Manifest, drop map[string]bool) (removed []string, kept Manifest) {
	kept = make(Manifest, 0, len(m))
	for _, e := range m {
		if drop[e.Path] {
			removed = append(removed, e.Path)
			continue
		}
		kept = append(kept, e)
	}

	sort.Strings(removed)
	return removed, kept
}

// dedupe returns the sorted paths with repeats collapsed, for the removals
// reported across several manifests
func dedupe(paths []string) []string {
	sort.Strings(paths)

	out := paths[:0:0]
	for i, p := range paths {
		if i > 0 && paths[i-1] == p {
			continue
		}
		out = append(out, p)
	}
	return out
}

// ManifestSource describes which manifest the package is reading, for the
// commands that report it.  It returns the base name of the manifest in use
// and whether that is a fallback, meaning the package's architecture has no
// manifest of its own and the plain one is standing in for it.
func (p *Pkg) ManifestSource() (name string, fallback bool) {
	name = filepath.Base(p.ManifestFile())
	return name, name != filepath.Base(p.WriteManifestFile())
}

// AddFileAllArches upserts the given repo-relative file into every manifest in
// the control dir rather than only the one for the package's own architecture.
//
// This is for the files that genuinely do not vary between architectures - a
// config file, a systemd unit, a script - which would otherwise have to be
// registered once per arch by setting each one in turn.  The sum written is the
// sum of the file on disk, which is why this must not be used for a
// cross-compiled binary: the bytes differ per arch, so recording the sum of
// whichever build happens to be in the tree would tell every other arch's
// manifest something untrue about its own contents.
//
// It returns the base names of the manifests it wrote, sorted.  Manifests that
// already record the correct sum are still reported, since the point of the
// command is to state where the file is registered.
func (p *Pkg) AddFileAllArches(relpath string) ([]string, error) {
	relpath, err := manifestPath(relpath)
	if err != nil {
		return nil, err
	}

	sum, err := Sum(p.Dir(relpath))
	if err != nil {
		return nil, err
	}

	// a doc file is registered at its destination in every manifest too, the
	// same as it would be for a single arch
	path, err := p.ManifestPathFor(relpath)
	if err != nil {
		return nil, err
	}

	// the manifest this package reads is included even when it does not exist
	// yet, so a single-arch package with nothing registered still gets an
	// entry rather than this silently doing nothing
	targets := p.ManifestFiles()
	if len(targets) == 0 {
		targets = []string{p.WriteManifestFile()}
	}

	var written []string
	for _, mf := range targets {
		m, err := ReadManifest(mf)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", filepath.Base(mf), err)
		}

		if err := writeManifestFile(mf, upsert(m, path, sum)); err != nil {
			return nil, err
		}

		written = append(written, filepath.Base(mf))
	}

	sort.Strings(written)
	return written, nil
}

// upsert returns the manifest with the given path recorded at the given sum,
// replacing an existing entry for it or appending a new one
func upsert(m Manifest, path, sum string) Manifest {
	for i, e := range m {
		if e.Path == path {
			m[i].Sum = sum
			return m
		}
	}

	return append(m, ManifestEntry{Sum: sum, Path: path})
}

// writeManifestFile writes a manifest to an explicit path, replacing any
// existing contents
func writeManifestFile(path string, m Manifest) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = m.Write(f)
	return err
}
