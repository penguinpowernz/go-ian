package ian

import (
	"os/exec"
	"strconv"

	"github.com/penguinpowernz/go-ian/debian/control"
	"github.com/penguinpowernz/go-ian/util/file"
)

// Pkg represents a debian package
type Pkg struct {
	ctrl control.Control
	dir  string
	errs []error
}

func NewPackage(dir string) (p *Pkg, err error) {
	p = new(Pkg)
	p.dir = dir
	p.ctrl, err = control.Read(p.CtrlFile())
	return
}

func (p *Pkg) Initialized() bool {
	return file.Exists(p.CtrlFile())
}

func (p *Pkg) Ctrl() *control.Control {
	return &p.ctrl
}

func (p *Pkg) Size() (string, error) {
	size, err := file.DirSize(p.Dir(), p.Excludes())
	if err != nil {
		size = size / 1024
	}

	return strconv.Itoa(size), nil
}

func (p *Pkg) BuildCommand() *exec.Cmd {
	return exec.Command(p.BuildFile(), p.Dir(), p.Ctrl().Version, p.Ctrl().Arch)
}
