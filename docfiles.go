package ian

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/penguinpowernz/go-ian/util/str"
)

// DocFilesFile returns the path to the package's docfiles list
func (p *Pkg) DocFilesFile() string {
	return p.CtrlDir("docfiles")
}

// DocFiles is the list of repo-relative paths that should be installed into
// the package's /usr/share/doc/<package> directory instead of at their own
// path in the repo.  It is read from DEBIAN/docfiles, one path per line.
//
// Each file's destination is the doc dir plus the file's base name, so
// "docs/guide.md" installs as "usr/share/doc/<package>/guide.md".  Flattening
// keeps the doc dir shallow, matching what the old root-file sweep produced.
type DocFiles []string

// ReadDocFiles parses a DEBIAN/docfiles list at the given path.  Blank lines
// and lines starting with '#' are ignored.  A missing file simply means the
// package has no doc files.
func ReadDocFiles(path string) (DocFiles, error) {
	data, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var d DocFiles
	for _, line := range str.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		d = append(d, line)
	}

	return d, nil
}

// DocFiles reads the package's DEBIAN/docfiles list
func (p *Pkg) DocFiles() (DocFiles, error) {
	return ReadDocFiles(p.DocFilesFile())
}

// Write writes the doc file list, one path per line, sorted
func (d DocFiles) Write(w io.Writer) (int, error) {
	sorted := make(DocFiles, len(d))
	copy(sorted, d)
	sort.Strings(sorted)

	var data []byte
	for _, p := range sorted {
		data = append(data, []byte(p+"\n")...)
	}

	return w.Write(data)
}

// WriteDocFiles writes the doc file list to the package's docfiles file,
// replacing the listed paths while keeping any comment header already in the
// file, so that the explanation written by `ian init` survives edits.
func (p *Pkg) WriteDocFiles(d DocFiles) error {
	header, err := p.docFilesHeader()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(p.DocFilesFile(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if header != "" {
		if _, err := f.WriteString(header); err != nil {
			return err
		}
	}

	_, err = d.Write(f)
	return err
}

// docFilesHeader returns the run of comment and blank lines at the top of the
// docfiles file, so it can be preserved when rewriting the list
func (p *Pkg) docFilesHeader() (string, error) {
	data, err := ioutil.ReadFile(p.DocFilesFile())
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	var header strings.Builder
	for _, line := range str.Lines(string(data)) {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
		header.WriteString(line + "\n")
	}

	return header.String(), nil
}

// DocDir returns the package's doc directory as a repo-relative path, e.g.
// "usr/share/doc/mypkg"
func (p *Pkg) DocDir() string {
	return filepath.Join("usr", "share", "doc", p.ctrl.Name)
}

// DocDest returns the repo-relative path that the given doc file installs to
// inside the package, i.e. the doc dir plus the file's base name
func (p *Pkg) DocDest(relpath string) string {
	return filepath.Join(p.DocDir(), filepath.Base(relpath))
}

// DocSources maps each doc file's destination path inside the package back to
// the repo-relative file it is copied from.  Staging and verification use this
// to find the source of a manifest entry that lives under the doc dir.
func (p *Pkg) DocSources() (map[string]string, error) {
	d, err := p.DocFiles()
	if err != nil {
		return nil, err
	}

	srcs := make(map[string]string, len(d))
	for _, src := range d {
		dst := p.DocDest(src)
		if existing, ok := srcs[dst]; ok && existing != src {
			return nil, fmt.Errorf("%s and %s both install as %s", existing, src, dst)
		}
		srcs[dst] = src
	}

	return srcs, nil
}

// SourceFor returns the repo-relative file that the given manifest path is
// staged from.  For most entries that is the path itself; for entries under the
// package's doc dir it is the doc file they were registered from.
func (p *Pkg) SourceFor(manifestPath string, docSrcs map[string]string) string {
	if src, ok := docSrcs[manifestPath]; ok {
		return src
	}
	return manifestPath
}

// AddDocFile registers the given repo-relative file as a doc file, recording it
// in DEBIAN/docfiles and adding its destination path and sum to the manifest so
// that it is included in the package and covered by verification.
func (p *Pkg) AddDocFile(relpath string) error {
	relpath, err := manifestPath(relpath)
	if err != nil {
		return err
	}

	// a doc file's destination lives under the doc dir, so registering a file
	// that is already in there would make it its own source
	if relpath == p.DocDir() || strings.HasPrefix(relpath, p.DocDir()+"/") {
		return fmt.Errorf("%s: already in the doc directory", relpath)
	}

	sum, err := Sum(p.Dir(relpath))
	if err != nil {
		return err
	}

	d, err := p.DocFiles()
	if err != nil {
		return err
	}

	dst := p.DocDest(relpath)
	for _, existing := range d {
		if existing == relpath {
			continue
		}
		if p.DocDest(existing) == dst {
			return fmt.Errorf("%s: %s already installs as %s", relpath, existing, dst)
		}
	}

	if !d.contains(relpath) {
		d = append(d, relpath)
		if err := p.WriteDocFiles(d); err != nil {
			return err
		}
	}

	// register the destination path in the manifest, since that is where the
	// file ends up in the package and what debsums will check
	return p.addEntry(dst, sum)
}

// RemoveDocFile unregisters the given doc file, dropping it from both the
// docfiles list and the manifest.  The file itself is left on disk.
func (p *Pkg) RemoveDocFile(relpath string) error {
	relpath, err := manifestPath(relpath)
	if err != nil {
		return err
	}

	d, err := p.DocFiles()
	if err != nil {
		return err
	}

	if !d.contains(relpath) {
		return fmt.Errorf("%s: not registered as a doc file", relpath)
	}

	kept := make(DocFiles, 0, len(d))
	for _, e := range d {
		if e != relpath {
			kept = append(kept, e)
		}
	}

	if err := p.WriteDocFiles(kept); err != nil {
		return err
	}

	_, err = p.RemoveFiles([]string{p.DocDest(relpath)})
	return err
}

// contains reports whether the list already holds the given path
func (d DocFiles) contains(path string) bool {
	for _, e := range d {
		if e == path {
			return true
		}
	}
	return false
}
