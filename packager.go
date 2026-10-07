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
		VerifyStaging,
		DpkgDebBuild,
		VerifyPackage,
	}
}

// BuildRequest is like a context object for packager strategies
// to make us of and share knowledge
type BuildRequest struct {
	Pkg     *Pkg
	Tmp     string
	debpath string
	Debug   bool

	// Quiet suppresses the md5sums listing printed while staging
	Quiet    bool
	Insecure bool

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
	Quiet            bool
	Insecure         bool
	SkipPackageCheck bool
}

// Build will create a debian package from the given control file and directory. It does this by
// verifying the files listed in DEBIAN/md5sums against their recorded sums, copying just those
// files to a temp dir, calculating the package size and calling dpkg-deb to build the package.
// The built package is then checked back against the manifest.  The path to the package and an
// error (if any) is returned.
func (pkgr Packager) Build(p *Pkg) (string, error) {
	return pkgr.BuildWithOpts(p, BuildOpts{Debug: Debug})
}

// BuildWithOpts does the same as build but with specifc options
func (pkgr Packager) BuildWithOpts(p *Pkg, opts BuildOpts) (string, error) {
	br := &BuildRequest{
		Pkg:              p,
		debpath:          opts.Outpath,
		Debug:            opts.Debug,
		Quiet:            opts.Quiet,
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

	if br.Debug {
		fmt.Fprintf(os.Stderr, "installed size: %s kB (%d bytes)\n", br.Pkg.ctrl.Size, b)
	}

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

	if br.Debug {
		os.Stderr.WriteString("comparing the packaged md5sums against the manifest\n")
		os.Stderr.WriteString("-------------------------------------------------\n")
		shippedSums := make(map[string]string, len(shipped))
		for _, e := range shipped {
			shippedSums[e.Path] = e.Sum
		}
		for _, e := range m {
			sum, ok := shippedSums[e.Path]
			switch {
			case !ok:
				fmt.Fprintf(os.Stderr, "  ABSENT    %s  %s\n", e.Sum, e.Path)
			case sum == e.Sum:
				fmt.Fprintf(os.Stderr, "  ok        %s  %s\n", e.Sum, e.Path)
			default:
				fmt.Fprintf(os.Stderr, "  DIFFERS   %s  %s (packaged %s)\n", e.Sum, e.Path, sum)
			}
		}
		os.Stderr.WriteString("-------------------------------------------------\n\n")
	}

	problems = append(problems, diffManifests(m, shipped)...)

	// 2. the files themselves, rehashed from the package contents
	fsysDir := filepath.Join(tmp, "fsys")
	cmd = exec.Command("/usr/bin/dpkg-deb", "--extract", br.debpath, fsysDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to extract %s: %s: %s", br.debpath, err, strings.TrimSpace(string(out)))
	}

	if br.Debug {
		os.Stderr.WriteString("rehashing the files extracted from " + br.debpath + "\n")
		os.Stderr.WriteString("-------------------------------------------------\n")
	}

	for _, e := range m {
		path := filepath.Join(fsysDir, e.Path)
		if !file.Exists(path) {
			if br.Debug {
				fmt.Fprintf(os.Stderr, "  MISSING   %s  %s\n", e.Sum, e.Path)
			}
			problems = append(problems, fmt.Sprintf("%s: registered but not in the built package", e.Path))
			continue
		}

		sum, err := Sum(path)
		if err != nil {
			if br.Debug {
				fmt.Fprintf(os.Stderr, "  ERROR     %s  %s (%s)\n", e.Sum, e.Path, err)
			}
			problems = append(problems, fmt.Sprintf("%s: %s", e.Path, err))
			continue
		}

		if sum != e.Sum {
			if br.Debug {
				fmt.Fprintf(os.Stderr, "  MISMATCH  %s  %s (got %s)\n", e.Sum, e.Path, sum)
			}
			problems = append(problems, fmt.Sprintf("%s: packaged file does not match the manifest (want %s, got %s)", e.Path, e.Sum, sum))
			continue
		}

		if br.Debug {
			fmt.Fprintf(os.Stderr, "  ok        %s  %s\n", sum, e.Path)
		}
	}

	if br.Debug {
		os.Stderr.WriteString("-------------------------------------------------\n\n")
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

		if br.Debug {
			fmt.Fprintf(os.Stderr, "  UNKNOWN   %s (in the package but not registered)\n", rel)
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
		fmt.Fprintf(os.Stderr, "verified %d file(s) in %s, %d problem(s)\n", len(m), br.debpath, len(problems))
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

	statuses, err := br.Pkg.Status()
	if err != nil {
		return fmt.Errorf("failed to check the manifest: %s", err)
	}

	if br.Debug {
		os.Stderr.WriteString("checking the repo files against " + br.Pkg.ManifestFile() + "\n")
		os.Stderr.WriteString("-------------------------------------------------\n")
	}

	var problems []string
	for _, st := range statuses {
		if br.Debug {
			// name the repo file that was summed, which for a doc file is not
			// the path it takes in the package
			shown := st.Path
			if st.IsDoc() {
				shown = fmt.Sprintf("%s -> %s", st.Source, st.Path)
			}

			switch st.State {
			case StateOK:
				fmt.Fprintf(os.Stderr, "  ok        %s  %s\n", st.Want, shown)
			case StateMissing:
				fmt.Fprintf(os.Stderr, "  MISSING   %s  %s\n", st.Want, shown)
			case StateError:
				fmt.Fprintf(os.Stderr, "  ERROR     %s  %s (%s)\n", st.Want, shown, st.Err)
			default:
				fmt.Fprintf(os.Stderr, "  MISMATCH  %s  %s (got %s)\n", st.Want, shown, st.Got)
			}
		}

		if msg := st.Problem(); msg != "" {
			problems = append(problems, msg)
		}
	}

	if br.Debug {
		os.Stderr.WriteString("-------------------------------------------------\n\n")
	}

	for _, p := range problems {
		os.Stderr.WriteString("WARNING: " + p + "\n")
	}

	if len(problems) > 0 && !br.Insecure {
		return fmt.Errorf("md5sum verification failed for %d file(s)", len(problems))
	}

	return nil
}

// VerifyStaging is a packaging step that checks the staged tree just before
// dpkg-deb is called, which is the last point at which a problem can be caught
// before it is sealed into a package.  It checks two things:
//
//  1. the staged DEBIAN/md5sums against the files staged alongside it, which is
//     what dpkg and debsums will see once the package is installed
//  2. the manifest in the repo against those same staged files, so that a file
//     altered between verification and staging is caught
//
// Like the other verification steps an insecure build only warns.
var VerifyStaging = func(br *BuildRequest) error {
	if br.Debug {
		os.Stderr.WriteString("\n\n*** VerifyStaging ***\n\n")
	}

	staged := &Pkg{dir: br.Tmp}

	stagedManifest, err := staged.Manifest()
	if err != nil {
		return fmt.Errorf("failed to read the staged manifest: %s", err)
	}

	repoManifest, err := br.Pkg.Manifest()
	if err != nil {
		return fmt.Errorf("failed to read manifest: %s", err)
	}

	var problems []string

	// 1. the staged manifest against the staged files
	if br.Debug {
		os.Stderr.WriteString("checking the staged files against the staged md5sums\n")
		os.Stderr.WriteString("-------------------------------------------------\n")
	}

	for _, e := range stagedManifest {
		path := filepath.Join(br.Tmp, e.Path)

		if !file.Exists(path) {
			if br.Debug {
				fmt.Fprintf(os.Stderr, "  MISSING   %s  %s\n", e.Sum, e.Path)
			}
			problems = append(problems, fmt.Sprintf("%s: listed in the staged md5sums but not staged", e.Path))
			continue
		}

		sum, err := Sum(path)
		if err != nil {
			if br.Debug {
				fmt.Fprintf(os.Stderr, "  ERROR     %s  %s (%s)\n", e.Sum, e.Path, err)
			}
			problems = append(problems, fmt.Sprintf("%s: %s", e.Path, err))
			continue
		}

		if sum != e.Sum {
			if br.Debug {
				fmt.Fprintf(os.Stderr, "  MISMATCH  %s  %s (got %s)\n", e.Sum, e.Path, sum)
			}
			problems = append(problems, fmt.Sprintf("%s: staged file does not match the staged md5sums (want %s, got %s)", e.Path, e.Sum, sum))
			continue
		}

		if br.Debug {
			fmt.Fprintf(os.Stderr, "  ok        %s  %s\n", sum, e.Path)
		}
	}

	if br.Debug {
		os.Stderr.WriteString("-------------------------------------------------\n\n")
	}

	// 2. the repo's manifest against the staged files, which also catches the
	// staged manifest having drifted from the committed one
	if br.Debug {
		os.Stderr.WriteString("checking the staged files against " + br.Pkg.ManifestFile() + "\n")
		os.Stderr.WriteString("-------------------------------------------------\n")
	}

	stagedSums := make(map[string]string, len(stagedManifest))
	for _, e := range stagedManifest {
		stagedSums[e.Path] = e.Sum
	}

	for _, e := range repoManifest {
		if sum, ok := stagedSums[e.Path]; !ok {
			problems = append(problems, fmt.Sprintf("%s: registered but missing from the staged md5sums", e.Path))
		} else if sum != e.Sum {
			problems = append(problems, fmt.Sprintf("%s: staged md5sums says %s, manifest says %s", e.Path, sum, e.Sum))
		}

		path := filepath.Join(br.Tmp, e.Path)

		if !file.Exists(path) {
			if br.Debug {
				fmt.Fprintf(os.Stderr, "  MISSING   %s  %s\n", e.Sum, e.Path)
			}
			problems = append(problems, fmt.Sprintf("%s: registered but not staged", e.Path))
			continue
		}

		sum, err := Sum(path)
		if err != nil {
			if br.Debug {
				fmt.Fprintf(os.Stderr, "  ERROR     %s  %s (%s)\n", e.Sum, e.Path, err)
			}
			problems = append(problems, fmt.Sprintf("%s: %s", e.Path, err))
			continue
		}

		if sum != e.Sum {
			if br.Debug {
				fmt.Fprintf(os.Stderr, "  MISMATCH  %s  %s (got %s)\n", e.Sum, e.Path, sum)
			}
			problems = append(problems, fmt.Sprintf("%s: staged file does not match the manifest (want %s, got %s)", e.Path, e.Sum, sum))
			continue
		}

		if br.Debug {
			fmt.Fprintf(os.Stderr, "  ok        %s  %s\n", sum, e.Path)
		}
	}

	if br.Debug {
		os.Stderr.WriteString("-------------------------------------------------\n\n")
	}

	// anything staged outside the control dir that nothing registered
	err = filepath.Walk(br.Tmp, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(br.Tmp, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			// the control dir is staged separately and is not package content
			if rel == "DEBIAN" {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		if _, ok := stagedSums[rel]; ok {
			return nil
		}

		if br.Debug {
			fmt.Fprintf(os.Stderr, "  UNKNOWN   %s (staged but not in the staged md5sums)\n", rel)
		}
		problems = append(problems, fmt.Sprintf("%s: staged but not in the staged md5sums", rel))
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk the staged tree: %s", err)
	}

	sort.Strings(problems)
	for _, p := range problems {
		os.Stderr.WriteString("WARNING: " + p + "\n")
	}

	if len(problems) > 0 && !br.Insecure {
		return fmt.Errorf("the staged tree does not match the manifest for %d file(s)", len(problems))
	}

	if br.Debug {
		fmt.Fprintf(os.Stderr, "verified %d staged file(s), %d problem(s)\n", len(stagedManifest), len(problems))
	}

	return nil
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

	// the sums that will be shipped, printed by default so a build leaves a
	// record of what went into the package
	if !br.Quiet {
		m.Write(os.Stderr)
	}

	return nil
}
