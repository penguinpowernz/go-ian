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

	// Start with an empty manifest: nothing is included in the package until
	// the developer registers files with `ian add`.
	if err := file.EmptyDotFile(pkg.ManifestFile()); err != nil {
		return err
	}

	if err := os.WriteFile(pkg.DocFilesFile(), []byte(defaultDocFiles), 0644); err != nil {
		return err
	}

	file.EmptyDotFile(pkg.Dir(".ianpush"))

	return nil
}

// defaultDocFiles is written to DEBIAN/docfiles on ian init.  Lines starting
// with '#' are comments, so the header explains the file without listing
// anything.
const defaultDocFiles = `# Files listed here are installed into usr/share/doc/<package> under their base
# name, rather than at their own path in the repo.  One path per line, relative
# to the package directory.  Lines starting with '#' are comments.
#
# Use ` + "`ian doc <file>`" + ` to add a file here, which also records its
# destination and md5 sum in DEBIAN/md5sums.
`

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
