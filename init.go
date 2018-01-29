package ian

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/penguinpowernz/go-ian/util/file"

	"github.com/penguinpowernz/go-ian/debian/control"
)

// IsInitialized determines if the directory is already initialized
func IsInitialized(dir string) bool {
	p := Pkg{dir: dir}
	return file.Exists(p.CtrlDir()) && file.Exists(p.CtrlFile())
}

// Initialize will turn the given directory into an ian repo
func Initialize(dir string) error {
	if IsInitialized(dir) {
		return fmt.Errorf("already initialized")
	}

	pkg := Pkg{dir: dir, ctrl: control.Default(filepath.Base(dir))}
	if err := os.MkdirAll(pkg.CtrlDir(), 0755); err != nil {
		return err
	}

	if err := pkg.ctrl.WriteFile(pkg.CtrlFile()); err != nil {
		return err
	}

	file.EmptyBashScript(pkg.CtrlDir("postinst"))
	file.EmptyBashScript(pkg.CtrlDir("prerm"))
	file.EmptyBashScript(pkg.CtrlDir("postrm"))
	file.EmptyBashScript(pkg.CtrlDir("preinst"))
	file.EmptyBashScript(pkg.CtrlDir("build"))

	file.EmptyDotFile(pkg.Dir(".ianignore"))
	file.EmptyDotFile(pkg.Dir(".ianpush"))

	return nil
}
