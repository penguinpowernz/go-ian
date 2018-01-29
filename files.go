package ian

import "path/filepath"

// CtrlFile returns the path to the control file in the given directory
func (p *Pkg) CtrlFile() string {
	return p.CtrlDir("control")
}

// CtrlDir returns the path to the control dir in the given directory. Optional
// extra paths will result in getting a path inside the control dir e.g. to the
// postinst file
func (p *Pkg) CtrlDir(paths ...string) string {
	return p.Dir(append([]string{"DEBIAN"}, paths...)...)
}

// Dir returns the path to the repo root directory. Optional
// extra paths will result in getting a path inside the dir
func (p *Pkg) Dir(paths ...string) string {
	paths = append([]string{p.dir}, paths...)
	return filepath.Join(paths...)
}

func (p *Pkg) DocPath(paths ...string) string {
	return p.Dir("usr", "share", "doc", p.ctrl.Name)
}

func (p *Pkg) BuildFile() string {
	return p.CtrlDir("build")
}

func (p *Pkg) DebFile() string {
	return p.Dir("pkg", p.ctrl.Filename())
}

func (p *Pkg) PushFile() string {
	return p.Dir(".ianpush")
}
