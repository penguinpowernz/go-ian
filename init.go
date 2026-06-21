package ian

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/penguinpowernz/go-ian/util/file"
	"github.com/penguinpowernz/go-ian/util/str"

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

	if mntr, ok := FindMaintainer(); ok {
		pkg.Ctrl().Maintainer = mntr
	}

	if err := pkg.ctrl.WriteFile(pkg.CtrlFile()); err != nil {
		return err
	}

	file.EmptyBashScript(pkg.CtrlDir("postinst"))
	file.EmptyBashScript(pkg.CtrlDir("prerm"))
	file.EmptyBashScript(pkg.CtrlDir("postrm"))
	file.EmptyBashScript(pkg.CtrlDir("preinst"))

	if err := os.WriteFile(pkg.Dir(".ianignore"), []byte(defaultIanignore), 0666); err != nil {
		return err
	}
	file.EmptyDotFile(pkg.Dir(".ianpush"))

	return nil
}

func FindMaintainer() (string, bool) {
	gcpath := filepath.Join(os.Getenv("HOME"), ".gitconfig")
	data, err := ioutil.ReadFile(gcpath)
	if err != nil {
		return "", false
	}

	lines := str.Lines(string(data))
	var name, email string

	for _, l := range lines {
		l = strings.TrimSpace(l)
		if parts := strings.SplitN(l, "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			switch key {
			case "name":
				name = val
			case "email":
				email = val
			}
		}
	}

	if name == "" || email == "" {
		return "", false
	}

	return fmt.Sprintf("%s <%s>", name, email), true
}

// defaultIanignore is written to .ianignore on ian init.
// Lines starting with '#' are treated as comments and ignored.
// The following patterns are always excluded regardless of this file:
//
//	.git, pkg, .gitignore, .ianpush, .ianignore, .gitkeep, ./.*
const defaultIanignore = `# Patterns listed here are excluded from the package in addition to the defaults.
# Lines starting with '#' are comments.
#
# Always excluded by default:
#   .git
#   pkg
#   .gitignore
#   .ianpush
#   .ianignore
#   .gitkeep
#   ./.* (all hidden files/folders at the root of IAN_DIR)
`
