package ian

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/penguinpowernz/go-ian/debian/control"
	"github.com/penguinpowernz/go-ian/util/file"
)

// Pkg represents a ian debian package with helpers for
// various operations around managing a debian package
// with ian
type Pkg struct {
	ctrl control.Control
	dir  string
	errs []error
}

// NewPackage returns a new Pkg object with the control
// file contained within, when given a directory
func NewPackage(dir string) (p *Pkg, err error) {
	p = new(Pkg)
	p.dir = dir
	p.ctrl, err = control.Read(p.CtrlFile())
	return
}

// Initialized will return true if the package has been initialized
func (p *Pkg) Initialized() bool {
	return file.Exists(p.CtrlFile())
}

// Ctrl returns the control file as a control object
func (p *Pkg) Ctrl() *control.Control {
	return &p.ctrl
}

// CtrlFiles returns a list of all the files in the control dir
func (p *Pkg) CtrlFiles() []string {
	m, _ := filepath.Glob(filepath.Join(p.CtrlDir(), "*"))
	return m
}

// Size returns the total size of the files to be included
// in the package, summed from the files listed in the manifest
func (p *Pkg) Size() (string, error) {
	m, err := p.Manifest()
	if err != nil {
		return "", err
	}

	docSrcs, err := p.DocSources()
	if err != nil {
		return "", err
	}

	var size int64
	for _, e := range m {
		fi, err := os.Stat(p.Dir(p.SourceFor(e.Path, docSrcs)))
		if err != nil {
			return "", err
		}
		size += fi.Size()
	}

	return strconv.Itoa(int(size) / 1024), nil
}
