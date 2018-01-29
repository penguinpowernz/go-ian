package ian

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/penguinpowernz/go-ian/util/checksum"
	"github.com/penguinpowernz/go-ian/util/file"
	"github.com/penguinpowernz/go-ian/util/str"
	"github.com/penguinpowernz/go-ian/util/tell"
)

// Debug is the default debug mode for the build options when they
// are not explicity specified with the BuildWithOpts() call
var Debug = false

// DefaultPackager returns a preconfigured packager
// using the default packaging steps/strategies
func DefaultPackager() (p Packager) {
	return Packager{
		StageFiles,
		CleanRoot,
		CalculateMD5Sums,
		CalculateSize,
		DpkgDebBuild,
	}
}

// BuildRequest is like a context object for packager strategies
// to make us of and share knowledge
type BuildRequest struct {
	pkg     *Pkg
	tmp     string
	debpath string
	Debug   bool
}

// CleanUp is run at the end of the package build to clean up
// any leftover resources
func (br *BuildRequest) CleanUp() {
	_ = os.RemoveAll(br.tmp)
}

// PackagerStrategy is a function that represents a strategy or
// stage in the packaging process
type PackagerStrategy func(br *BuildRequest) error

// Packager is a collection of packaging steps/strategies that
// can be used together to build a package
type Packager []PackagerStrategy

type BuildOpts struct {
	Outpath string
	Debug   bool
}

// Build will create a debian package from the given control file and directory. It does this by
// using rsync to copy the repo to a temp dir, excluded unwanted files and moving any files in the root
// of the package to a /usr/share/doc folder.  Then it calculates the package size, file checksums and
// calls dpkg-deb to build the package.  The path to the package and an error (if any) is returned.
func (pkgr Packager) Build(p *Pkg) (string, error) {
	return pkgr.BuildWithOpts(p, BuildOpts{Debug: Debug})
}

// BuildWithOpts does the same as build but with specifc options
func (pkgr Packager) BuildWithOpts(p *Pkg, opts BuildOpts) (string, error) {
	br := &BuildRequest{pkg: p, debpath: opts.Outpath, Debug: opts.Debug}

	for i, fn := range pkgr {
		err := fn(br)
		if err != nil {
			return "", fmt.Errorf("at step %d: %s", i+1, err)
		}
	}

	br.CleanUp()
	return br.debpath, nil
}

// DpkgDebBuild is a packaging step that builds the package using dpkg-deb
var DpkgDebBuild = func(br *BuildRequest) error {
	if br.debpath == "" {
		br.debpath = br.pkg.Dir("pkg")
	}

	if err := os.MkdirAll(br.debpath, 0755); err != nil {
		return fmt.Errorf("failed to make package dir at %s: %s", br.debpath, err)
	}

	br.debpath = filepath.Join(br.debpath, br.pkg.ctrl.Filename())

	cmd := exec.Command("/usr/bin/fakeroot", "dpkg-deb", "-b", br.tmp, br.debpath)
	if br.Debug {
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build package %s from %s: %s", br.debpath, br.tmp, err)
	}

	return nil
}

// CalculateSize of a directory using du, excluding any given paths
var CalculateSize = func(br *BuildRequest) error {
	b, err := file.DirSize(br.tmp, br.pkg.Excludes())
	if err != nil {
		return fmt.Errorf("failed to calculate package size: %s", err)
	}

	br.pkg.ctrl.Size = strconv.Itoa(b / 1024)
	br.pkg.ctrl.WriteFile((&Pkg{dir: br.tmp}).CtrlFile())
	br.pkg.ctrl.WriteFile(br.pkg.CtrlFile())
	return nil
}

// CalculateMD5Sums is a packaging step that calculates the file sums
var CalculateMD5Sums = func(br *BuildRequest) error {
	sums := filepath.Join(br.pkg.CtrlDir(), "md5sums")
	if err := checksum.MD5(br.tmp, sums); err != nil {
		return fmt.Errorf("failed to generate md5sums: %s", err)
	}

	return nil
}

// StageFiles is a packaging step that stages the package files to a
// temporary directory to work from
var StageFiles = func(br *BuildRequest) error {
	var err error
	br.tmp, err = ioutil.TempDir("/tmp", "go-ian")
	if err != nil {
		return fmt.Errorf("couldn't make tmp dir: %s", err)
	}

	args := []string{"-rav"}
	for _, s := range br.pkg.Excludes() {
		if s == "" {
			continue
		}
		args = append(args, fmt.Sprintf("--exclude=%s", s))
	}
	args = append(args, br.pkg.Dir()+"/", br.tmp)

	cmd := exec.Command("/usr/bin/rsync", args...)
	if br.Debug {
		tell.Debugf("running: %s", str.CommandString(cmd))
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stderr
	}
	return cmd.Run()
}

// CleanRoot is a packaging step to clean the root folder of the
// package so that the target root file system is not polluted
var CleanRoot = func(br *BuildRequest) error {
	list, err := file.ListFilesIn(br.tmp)
	if err != nil {
		return fmt.Errorf("failed to find root files: %s", err)
	}

	return file.MoveFiles(list, br.tmp)
}
