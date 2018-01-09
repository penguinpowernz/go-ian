package ian

import "path/filepath"

// ControlFile returns the path to the control file in the given directory
func ControlFile(dir string) string {
	return ControlDir(dir, "control")
}

// ControlDir returns the path to the control dir in the given directory. Optional
// extra paths will result in getting a path inside the control dir e.g. to the
// postinst file
func ControlDir(dir string, paths ...string) string {
	paths = append([]string{dir, "DEBIAN"}, paths...)
	return filepath.Join(paths...)
}
