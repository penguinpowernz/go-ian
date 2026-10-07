package ian

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/penguinpowernz/go-ian/util/file"
)

// Debug is the default debug mode for the build options when they
// are not explicity specified with the BuildWithOpts() call
var Debug = false

// MaintainerScripts are the control files that dpkg executes, and so are the
// only ones staged as 0755.  Every other control file is data that dpkg only
// reads (control, md5sums, conffiles, templates...) and is staged as 0644.
var MaintainerScripts = map[string]bool{
	"preinst":  true,
	"postinst": true,
	"prerm":    true,
	"postrm":   true,
	"config":   true,
}

// DefaultPackager returns a preconfigured packager
// using the default packaging steps/strategies
func DefaultPackager() (p Packager) {
	return Packager{
		VerifyManifest,
		StageFiles,
		CalculateSize,
		PrintPackageTree,
		DpkgDebBuild,
		VerifyPackage,
	}
}

// BuildRequest is like a context object for packager strategies
// to make us of and share knowledge
type BuildRequest struct {
	Pkg          *Pkg
	Tmp          string
	debpath      string
	Debug        bool
	PrintMD5Sums bool
	Insecure     bool

	// SkipPackageCheck skips the post-build verification of the .deb entirely,
	// rather than downgrading its failures to warnings the way Insecure does
	SkipPackageCheck bool
}

// CleanUp is run at the end of the package build to clean up
// any leftover resources
func (br *BuildRequest) CleanUp() {
	_ = os.RemoveAll(br.Tmp)
}

// PackagerStrategy is a function that represents a strategy or
// stage in the packaging process
type PackagerStrategy func(br *BuildRequest) error

// Packager is a collection of packaging steps/strategies that
// can be used together to build a package
type Packager []PackagerStrategy

type BuildOpts struct {
	Outpath          string
	Debug            bool
	PrintMD5Sums     bool
	Insecure         bool
	SkipPackageCheck bool
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
	br := &BuildRequest{
		Pkg:              p,
		debpath:          opts.Outpath,
		Debug:            opts.Debug,
		PrintMD5Sums:     opts.PrintMD5Sums,
		Insecure:         opts.Insecure,
		SkipPackageCheck: opts.SkipPackageCheck,
	}

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
	for _, fn := range file.Glob(br.Tmp, "**") {
		os.Stderr.WriteString(strings.Replace(fn, br.Tmp+"/", "", -1) + "\n")
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
		data, err := os.ReadFile(br.Pkg.CtrlFile())
		if err != nil {
			os.Stderr.WriteString("ERROR: failed to read the control file from " + br.Pkg.dir + "\n")
		}
		os.Stderr.Write(data)
		os.Stderr.WriteString("-------------------------------------------------\n\n")
	}

	if br.debpath == "" {
		br.debpath = br.Pkg.Dir("pkg")
	}

	if err := os.MkdirAll(br.debpath, 0755); err != nil {
		return fmt.Errorf("failed to make package dir at %s: %s", br.debpath, err)
	}

	// ensure correct perms on the staged control dir
	stagedCtrlDir := filepath.Join(br.Tmp, "DEBIAN")
	if err := os.Chmod(stagedCtrlDir, 0755); err != nil {
		return fmt.Errorf("failed to set the proper perms on the control dir")
	}

	// ensure correct perms on the staged control files: the maintainer scripts
	// need to be executable, while data files like control, md5sums and
	// conffiles are 0644 as dpkg only ever reads them
	for _, fpath := range file.Glob(stagedCtrlDir, "*") {
		mode := os.FileMode(0644)
		if MaintainerScripts[filepath.Base(fpath)] {
			mode = 0755
		}
		if err := os.Chmod(fpath, mode); err != nil {
			return fmt.Errorf("failed to set the proper perms on the control file %s", fpath)
		}
	}

	br.debpath = filepath.Join(br.debpath, br.Pkg.ctrl.Filename())

	cmd := exec.Command("/usr/bin/fakeroot", "dpkg-deb", "-b", "-Zgzip", br.Tmp, br.debpath)
	if br.Debug {
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build package %s from %s: %s", br.debpath, br.Tmp, err)
	}

	return nil
}

// CalculateSize of the staged package directory using du
var CalculateSize = func(br *BuildRequest) error {
	if br.Debug {
		os.Stderr.WriteString("\n\n*** CalculateSize ***\n\n")
	}

	b, err := file.DirSize(br.Tmp)
	if err != nil {
		return fmt.Errorf("failed to calculate package size: %s", err)
	}

	br.Pkg.ctrl.Size = strconv.Itoa(b / 1024)
	br.Pkg.ctrl.WriteFile((&Pkg{dir: br.Tmp}).CtrlFile())
	br.Pkg.ctrl.WriteFile(br.Pkg.CtrlFile())
	return nil
}

// VerifyPackage is a packaging step that checks the built package against the
// manifest, after dpkg-deb has produced it.  It does two things:
//
//  1. reads the md5sums control file back out of the .deb and compares it to
//     the committed manifest, catching anything that went wrong between the
//     manifest and what was shipped
//  2. extracts the package contents and rehashes every file, so a file whose
//     sum changed between being verified and being packaged is caught
//
// This is the end to end check that what was released matches what was
// registered.  As with the pre-build verification, an insecure build only warns.
var VerifyPackage = func(br *BuildRequest) error {
	if br.Debug {
		os.Stderr.WriteString("\n\n*** VerifyPackage ***\n\n")
	}

	if br.SkipPackageCheck {
		if br.Debug {
			os.Stderr.WriteString("WARNING: skipped final package check\n")
		}
		return nil
	}

	m, err := br.Pkg.Manifest()
	if err != nil {
		return fmt.Errorf("failed to read manifest: %s", err)
	}

	tmp, err := ioutil.TempDir("/tmp", "go-ian-verify")
	if err != nil {
		return fmt.Errorf("couldn't make tmp dir: %s", err)
	}
	defer os.RemoveAll(tmp)

	var problems []string

	// 1. the md5sums shipped inside the package
	ctrlDir := filepath.Join(tmp, "control")
	cmd := exec.Command("/usr/bin/dpkg-deb", "--control", br.debpath, ctrlDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to read control files from %s: %s: %s", br.debpath, err, strings.TrimSpace(string(out)))
	}

	shipped, err := ReadManifest(filepath.Join(ctrlDir, "md5sums"))
	if err != nil {
		return fmt.Errorf("failed to read the packaged md5sums: %s", err)
	}

	problems = append(problems, diffManifests(m, shipped)...)

	// 2. the files themselves, rehashed from the package contents
	fsysDir := filepath.Join(tmp, "fsys")
	cmd = exec.Command("/usr/bin/dpkg-deb", "--extract", br.debpath, fsysDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to extract %s: %s: %s", br.debpath, err, strings.TrimSpace(string(out)))
	}

	for _, e := range m {
		path := filepath.Join(fsysDir, e.Path)
		if !file.Exists(path) {
			problems = append(problems, fmt.Sprintf("%s: registered but not in the built package", e.Path))
			continue
		}

		sum, err := Sum(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %s", e.Path, err))
			continue
		}

		if sum != e.Sum {
			problems = append(problems, fmt.Sprintf("%s: packaged file does not match the manifest (want %s, got %s)", e.Path, e.Sum, sum))
		}
	}

	// anything in the package that was never registered should not be there
	err = filepath.Walk(fsysDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !info.Mode().IsRegular() {
			return err
		}

		rel, err := filepath.Rel(fsysDir, path)
		if err != nil {
			return err
		}

		for _, e := range m {
			if e.Path == rel {
				return nil
			}
		}

		problems = append(problems, fmt.Sprintf("%s: in the built package but not registered", rel))
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk the built package: %s", err)
	}

	for _, p := range problems {
		os.Stderr.WriteString("WARNING: " + p + "\n")
	}

	if len(problems) > 0 && !br.Insecure {
		return fmt.Errorf("the built package does not match the manifest for %d file(s)", len(problems))
	}

	if br.Debug {
		os.Stderr.WriteString(fmt.Sprintf("verified %d file(s) in %s\n", len(m), br.debpath))
	}

	return nil
}

// diffManifests compares the committed manifest against the one shipped inside
// the built package, returning a problem for each difference
func diffManifests(committed, shipped Manifest) []string {
	var problems []string

	shippedSums := make(map[string]string, len(shipped))
	for _, e := range shipped {
		shippedSums[e.Path] = e.Sum
	}

	committedSums := make(map[string]string, len(committed))
	for _, e := range committed {
		committedSums[e.Path] = e.Sum
	}

	for _, e := range committed {
		sum, ok := shippedSums[e.Path]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: missing from the packaged md5sums", e.Path))
		case sum != e.Sum:
			problems = append(problems, fmt.Sprintf("%s: packaged md5sums says %s, manifest says %s", e.Path, sum, e.Sum))
		}
	}

	for _, e := range shipped {
		if _, ok := committedSums[e.Path]; !ok {
			problems = append(problems, fmt.Sprintf("%s: in the packaged md5sums but not the manifest", e.Path))
		}
	}

	sort.Strings(problems)
	return problems
}

// VerifyManifest is a packaging step that checks every file in the manifest
// against its recorded md5 sum before staging.  Unless the build is insecure
// it fails on any mismatch or missing file; when insecure it only warns.
var VerifyManifest = func(br *BuildRequest) error {
	if br.Debug {
		os.Stderr.WriteString("\n\n*** VerifyManifest ***\n\n")
	}

	m, err := br.Pkg.Manifest()
	if err != nil {
		return fmt.Errorf("failed to read manifest: %s", err)
	}

	if len(m) == 0 {
		return fmt.Errorf("no files registered in %s, use `ian add <file>` to register them", br.Pkg.ManifestFile())
	}

	problems, err := br.Pkg.Verify(br.Insecure)
	for _, p := range problems {
		os.Stderr.WriteString("WARNING: " + p + "\n")
	}
	return err
}

// StageFiles is a packaging step that stages the package files to a
// temporary directory to work from.  Only files listed in the manifest are
// copied (each at its repo-relative path), followed by the DEBIAN control
// files.  Nothing else from the repo is included.
var StageFiles = func(br *BuildRequest) error {
	if br.Debug {
		os.Stderr.WriteString("\n\n*** StageFiles ***\n\n")
	}

	var err error
	br.Tmp, err = ioutil.TempDir("/tmp", "go-ian")
	if err != nil {
		return fmt.Errorf("couldn't make tmp dir: %s", err)
	}

	if br.Debug {
		os.Stderr.WriteString("\nStaging files to " + br.Tmp + "\n")
		os.Stderr.WriteString("-------------------------------------------------\n")
	}

	m, err := br.Pkg.Manifest()
	if err != nil {
		return fmt.Errorf("failed to read manifest: %s", err)
	}

	// doc files are copied from their place in the repo to the package's doc
	// dir, so their manifest path is not where the source file lives
	docSrcs, err := br.Pkg.DocSources()
	if err != nil {
		return fmt.Errorf("failed to read doc files: %s", err)
	}

	for _, e := range m {
		src := br.Pkg.Dir(br.Pkg.SourceFor(e.Path, docSrcs))
		if !file.Exists(src) {
			// already warned about by VerifyManifest in insecure mode; skip
			continue
		}
		dst := filepath.Join(br.Tmp, e.Path)
		if err := file.CopyFile(src, dst); err != nil {
			return fmt.Errorf("failed to stage %s: %s", e.Path, err)
		}
		if br.Debug {
			os.Stderr.WriteString(e.Path + "\n")
		}
	}

	// stage the DEBIAN control files, including the md5sums manifest itself,
	// which is copied in verbatim so the package ships exactly the sums that
	// were committed and debsums works normally
	for _, fpath := range br.Pkg.CtrlFiles() {
		dst := filepath.Join(br.Tmp, "DEBIAN", filepath.Base(fpath))
		if err := file.CopyFile(fpath, dst); err != nil {
			return fmt.Errorf("failed to stage control file %s: %s", fpath, err)
		}
	}

	if br.Debug {
		os.Stderr.WriteString("-------------------------------------------------\n\n")
	}

	if br.PrintMD5Sums || br.Debug {
		os.Stderr.WriteString("\nMD5SUMS\n")
		os.Stderr.WriteString("-------------------------------------------------\n")
		m.Write(os.Stderr)
		os.Stderr.WriteString("-------------------------------------------------\n\n")
	}

	return nil
}
