package ian

import (
	"os/exec"
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

// Size returns the total size of the files to be included
// in the package
func (p *Pkg) Size() (string, error) {
	size, err := file.DirSize(p.Dir(), p.Excludes())
	if err != nil {
		size = size / 1024
	}

	return strconv.Itoa(size), nil
}

// BuildCommand returns the command to run in order to run
// the packages build script
func (p *Pkg) BuildCommand() *exec.Cmd {
	return exec.Command(p.BuildFile(), p.Dir(), p.Ctrl().Version, p.Ctrl().Arch)
}
