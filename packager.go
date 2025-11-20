package ian

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/penguinpowernz/go-ian/util/file"
	"github.com/penguinpowernz/go-ian/util/str"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/penguinpowernz/md5walk"
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
		PrintPackageTree,
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

var PrintPackageTree = func(br *BuildRequest) error {
	if !Debug {
		return nil
	}

	os.Stderr.WriteString("\nResultant Package Tree\n")
	os.Stderr.WriteString("-------------------------------------------------\n")
	for _, fn := range file.Glob(br.tmp, "**") {
		os.Stderr.WriteString(strings.Replace(fn, br.tmp+"/", "", -1) + "\n")
	}
	os.Stderr.WriteString("-------------------------------------------------\n\n")

	return nil
}

// DpkgDebBuild is a packaging step that builds the package using dpkg-deb
var DpkgDebBuild = func(br *BuildRequest) error {
	if br.Debug {
		os.Stderr.WriteString("\n\n*** DpkgDebBuild ***\n\n")
	}

	if br.Debug {
		os.Stderr.WriteString("\nControl file that will be used for the package\n")
		os.Stderr.WriteString("-------------------------------------------------\n")
		data, err := os.ReadFile(br.pkg.CtrlFile())
		if err != nil {
			os.Stderr.WriteString("ERROR: failed to read the control file from " + br.pkg.dir + "\n")
		}
		os.Stderr.Write(data)
		os.Stderr.WriteString("-------------------------------------------------\n\n")
	}

	if br.debpath == "" {
		br.debpath = br.pkg.Dir("pkg")
	}

	if err := os.MkdirAll(br.debpath, 0755); err != nil {
		return fmt.Errorf("failed to make package dir at %s: %s", br.debpath, err)
	}

	// ensure correct perms on ctrl dir
	if err := os.Chmod(br.pkg.dir, 0755); err != nil {
		return fmt.Errorf("failed to set the proper perms on the control dir")
	}

	// ensure correct perms on ctrl files
	for _, fpath := range br.pkg.CtrlFiles() {
		if err := os.Chmod(fpath, 0755); err != nil {
			return fmt.Errorf("failed to set the proper perms on the control file %s", fpath)
		}
	}

	br.debpath = filepath.Join(br.debpath, br.pkg.ctrl.Filename())

	cmd := exec.Command("/usr/bin/fakeroot", "dpkg-deb", "-b", "-Zgzip", br.tmp, br.debpath)
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
	if br.Debug {
		os.Stderr.WriteString("\n\n*** CalculateSize ***\n\n")
	}

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
	if br.Debug {
		os.Stderr.WriteString("\n\n*** CalculateMD5Sums ***\n\n")
	}

	outfile := (&Pkg{dir: br.tmp}).CtrlDir("md5sums")
	sums, err := md5walk.Walk(br.tmp)
	if err != nil {
		return fmt.Errorf("failed to generate md5sums: %s", err)
	}

	f, err := os.OpenFile(outfile, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("failed to write md5sums: %s", err)
	}

	_, err = sums.Write(f)

	if br.Debug {
		os.Stderr.WriteString("\nMD5SUMS\n")
		os.Stderr.WriteString("-------------------------------------------------\n")
		sums.Write(os.Stderr)
		os.Stderr.WriteString("-------------------------------------------------\n\n")
	}

	return err
}

// StageFiles is a packaging step that stages the package files to a
// temporary directory to work from
var StageFiles = func(br *BuildRequest) error {
	if br.Debug {
		os.Stderr.WriteString("\n\n*** StageFiles ***\n\n")
	}

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
		os.Stderr.WriteString("\nStaging files to " + br.tmp + "\n")
		os.Stderr.WriteString("-------------------------------------------------\n")
		tell.Debugf("running: %s", str.CommandString(cmd))
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stderr
	}

	err = cmd.Run()

	if br.Debug {
		os.Stderr.WriteString("-------------------------------------------------\n\n")
	}

	return err
}

// CleanRoot is a packaging step to clean the root folder of the
// package so that the target root file system is not polluted
var CleanRoot = func(br *BuildRequest) error {
	if br.Debug {
		os.Stderr.WriteString("\n\n*** CleanRoot ***\n\n")
	}

	list, err := file.ListFilesIn(br.tmp)
	if err != nil {
		return fmt.Errorf("failed to find root files: %s", err)
	}

	docpath := filepath.Join(br.tmp, "usr", "share", "doc", br.pkg.ctrl.Name)
	if err := os.MkdirAll(docpath, 0755); err != nil {
		return fmt.Errorf("failed to create the doc path %s: %s", docpath, err)
	}

	return file.MoveFiles(list, docpath)
}
