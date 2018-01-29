package ian

import (
	"fmt"

	"github.com/penguinpowernz/go-ian/util/file"

	"github.com/penguinpowernz/go-ian/debian/control"
)

// IsInitialized determines if the directory is already initialized
func IsInitialized(dir string) bool {
	p := Pkg{dir: dir}
	return file.Exists(p.CtrlDir()) && file.Exists(p.CtrlFile())
}

func Initialize(dir string) error {
	if IsInitialized(dir) {
		return fmt.Errorf("already initialized")
	}

	pkg := Pkg{dir: dir}
	control.Default().WriteFile(pkg.CtrlFile())

	file.EmptyBashScript(pkg.CtrlDir("postinst"))
	file.EmptyBashScript(pkg.CtrlDir("prerm"))
	file.EmptyBashScript(pkg.CtrlDir("postrm"))
	file.EmptyBashScript(pkg.CtrlDir("preinst"))
	file.EmptyBashScript(pkg.CtrlDir("build"))

	file.EmptyDotFile(pkg.Dir(".ianignore"))
	file.EmptyDotFile(pkg.Dir(".ianpush"))

	return nil
}
