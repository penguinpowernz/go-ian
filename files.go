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

// DocPath returns the path to the packages doc folder
func (p *Pkg) DocPath() string {
	return p.Dir("usr", "share", "doc", p.ctrl.Name)
}

// BuildFile returns the filepath to the build script file
func (p *Pkg) BuildFile() string {
	return p.CtrlDir("build")
}

// DebFile returns the filepath to where the debian package
// should be placed after building it
func (p *Pkg) DebFile() string {
	return p.Dir("pkg", p.ctrl.Filename())
}

// PushFile returns the filepath to the push file for this
// package
func (p *Pkg) PushFile() string {
	return p.Dir(".ianpush")
}
