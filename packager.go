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

	// dbg formats the debug output for the steps of this build
	dbg *debug
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
		dbg:              newDebug(opts.Debug),
	}

	for i, fn := range pkgr {
		err := fn(br)
		if err != nil {
			br.dbg.EndSection()
			return "", fmt.Errorf("at step %d: %s", i+1, err)
		}
	}

	br.dbg.EndSection()

	br.CleanUp()
	return br.debpath, nil
}

// DpkgDebBuild is a packaging step that builds the package using dpkg-deb
var DpkgDebBuild = func(br *BuildRequest) error {
	br.dbg.Step("building the package with dpkg-deb")

	if br.Debug {
		br.dbg.Section("control file that will be used for the package")
		data, err := os.ReadFile(br.Pkg.CtrlFile())
		if err != nil {
			br.dbg.Printf("ERROR: failed to read the control file from %s", br.Pkg.dir)
		}
		br.dbg.Write(data)
		br.dbg.EndSection()
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
		br.dbg.Section("%s", strings.Join(cmd.Args, " "))
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build package %s from %s: %s", br.debpath, br.Tmp, err)
	}

	br.dbg.Summary("built %s", br.debpath)

	return nil
}

// CalculateSize of the staged package directory using du
var CalculateSize = func(br *BuildRequest) error {
	br.dbg.Step("calculating the installed size")

	b, err := file.DirSize(br.Tmp)
	if err != nil {
		return fmt.Errorf("failed to calculate package size: %s", err)
	}

	br.Pkg.ctrl.Size = strconv.Itoa(b / 1024)
	br.Pkg.ctrl.WriteFile((&Pkg{dir: br.Tmp}).CtrlFile())
	br.Pkg.ctrl.WriteFile(br.Pkg.CtrlFile())

	br.dbg.Summary("installed size: %s kB (%d bytes)", br.Pkg.ctrl.Size, b)

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
	br.dbg.Step("verifying the built package")

	if br.SkipPackageCheck {
		br.dbg.Printf("WARNING: skipped final package check")
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
		br.dbg.Section("the packaged md5sums against %s", br.Pkg.ManifestFile())
		shippedSums := make(map[string]string, len(shipped))
		for _, e := range shipped {
			shippedSums[e.Path] = e.Sum
		}
		for _, e := range m {
			sum, ok := shippedSums[e.Path]
			switch {
			case !ok:
				br.dbg.File("ABSENT", e.Sum, e.Path, "")
			case sum == e.Sum:
				br.dbg.File("ok", e.Sum, e.Path, "")
			default:
				br.dbg.File("DIFFERS", e.Sum, e.Path, "packaged %s", sum)
			}
		}
		br.dbg.EndSection()
	}

	problems = append(problems, diffManifests(m, shipped)...)

	// 2. the files themselves, rehashed from the package contents
	fsysDir := filepath.Join(tmp, "fsys")
	cmd = exec.Command("/usr/bin/dpkg-deb", "--extract", br.debpath, fsysDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to extract %s: %s: %s", br.debpath, err, strings.TrimSpace(string(out)))
	}

	br.dbg.Section("files rehashed from %s", br.debpath)

	for _, e := range m {
		path := filepath.Join(fsysDir, e.Path)
		if !file.Exists(path) {
			br.dbg.File("MISSING", e.Sum, e.Path, "")
			problems = append(problems, fmt.Sprintf("%s: registered but not in the built package", e.Path))
			continue
		}

		sum, err := Sum(path)
		if err != nil {
			br.dbg.File("ERROR", e.Sum, e.Path, "%s", err)
			problems = append(problems, fmt.Sprintf("%s: %s", e.Path, err))
			continue
		}

		if sum != e.Sum {
			br.dbg.File("MISMATCH", e.Sum, e.Path, "got %s", sum)
			problems = append(problems, fmt.Sprintf("%s: packaged file does not match the manifest (want %s, got %s)", e.Path, e.Sum, sum))
			continue
		}

		br.dbg.File("ok", sum, e.Path, "")
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

		br.dbg.File("UNKNOWN", "", rel, "in the package but not registered")

		problems = append(problems, fmt.Sprintf("%s: in the built package but not registered", rel))
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk the built package: %s", err)
	}

	br.dbg.EndSection()

	for _, p := range problems {
		os.Stderr.WriteString("WARNING: " + p + "\n")
	}

	if len(problems) > 0 && !br.Insecure {
		return fmt.Errorf("the built package does not match the manifest for %d file(s)", len(problems))
	}

	br.dbg.Summary("verified %d file(s) in %s, %d problem(s)", len(m), br.debpath, len(problems))

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
	br.dbg.Step("verifying the manifest")

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

	br.dbg.Section("repo files against %s", br.Pkg.ManifestFile())

	var problems []string
	for _, st := range statuses {
		// name the repo file that was summed, which for a doc file is not
		// the path it takes in the package
		shown := st.Path
		if st.IsDoc() {
			shown = fmt.Sprintf("%s -> %s", st.Source, st.Path)
		}

		switch st.State {
		case StateOK:
			br.dbg.File("ok", st.Want, shown, "")
		case StateMissing:
			br.dbg.File("MISSING", st.Want, shown, "")
		case StateError:
			br.dbg.File("ERROR", st.Want, shown, "%s", st.Err)
		default:
			br.dbg.File("MISMATCH", st.Want, shown, "got %s", st.Got)
		}

		if msg := st.Problem(); msg != "" {
			problems = append(problems, msg)
		}
	}

	br.dbg.EndSection()

	for _, p := range problems {
		os.Stderr.WriteString("WARNING: " + p + "\n")
	}

	if len(problems) > 0 && !br.Insecure {
		return fmt.Errorf("md5sum verification failed for %d file(s)", len(problems))
	}

	br.dbg.Summary("checked %d registered file(s), %d problem(s)", len(m), len(problems))

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
	br.dbg.Step("verifying the staged tree")

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
	br.dbg.Section("staged files against the staged DEBIAN/md5sums")

	for _, e := range stagedManifest {
		path := filepath.Join(br.Tmp, e.Path)

		if !file.Exists(path) {
			br.dbg.File("MISSING", e.Sum, e.Path, "")
			problems = append(problems, fmt.Sprintf("%s: listed in the staged md5sums but not staged", e.Path))
			continue
		}

		sum, err := Sum(path)
		if err != nil {
			br.dbg.File("ERROR", e.Sum, e.Path, "%s", err)
			problems = append(problems, fmt.Sprintf("%s: %s", e.Path, err))
			continue
		}

		if sum != e.Sum {
			br.dbg.File("MISMATCH", e.Sum, e.Path, "got %s", sum)
			problems = append(problems, fmt.Sprintf("%s: staged file does not match the staged md5sums (want %s, got %s)", e.Path, e.Sum, sum))
			continue
		}

		br.dbg.File("ok", sum, e.Path, "")
	}

	// 2. the repo's manifest against the staged files, which also catches the
	// staged manifest having drifted from the committed one
	br.dbg.Section("staged files against %s", br.Pkg.ManifestFile())

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
			br.dbg.File("MISSING", e.Sum, e.Path, "")
			problems = append(problems, fmt.Sprintf("%s: registered but not staged", e.Path))
			continue
		}

		sum, err := Sum(path)
		if err != nil {
			br.dbg.File("ERROR", e.Sum, e.Path, "%s", err)
			problems = append(problems, fmt.Sprintf("%s: %s", e.Path, err))
			continue
		}

		if sum != e.Sum {
			br.dbg.File("MISMATCH", e.Sum, e.Path, "got %s", sum)
			problems = append(problems, fmt.Sprintf("%s: staged file does not match the manifest (want %s, got %s)", e.Path, e.Sum, sum))
			continue
		}

		br.dbg.File("ok", sum, e.Path, "")
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

		br.dbg.File("UNKNOWN", "", rel, "staged but not in the staged md5sums")
		problems = append(problems, fmt.Sprintf("%s: staged but not in the staged md5sums", rel))
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk the staged tree: %s", err)
	}

	br.dbg.EndSection()

	sort.Strings(problems)
	for _, p := range problems {
		os.Stderr.WriteString("WARNING: " + p + "\n")
	}

	if len(problems) > 0 && !br.Insecure {
		return fmt.Errorf("the staged tree does not match the manifest for %d file(s)", len(problems))
	}

	br.dbg.Summary("verified %d staged file(s), %d problem(s)", len(stagedManifest), len(problems))

	return nil
}

// StageFiles is a packaging step that stages the package files to a
// temporary directory to work from.  Only files listed in the manifest are
// copied (each at its repo-relative path), followed by the DEBIAN control
// files.  Nothing else from the repo is included.
var StageFiles = func(br *BuildRequest) error {
	br.dbg.Step("staging the package files")

	var err error
	br.Tmp, err = ioutil.TempDir("/tmp", "go-ian")
	if err != nil {
		return fmt.Errorf("couldn't make tmp dir: %s", err)
	}

	br.dbg.Section("files staged to %s", br.Tmp)

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

	var staged, skipped int
	for _, e := range m {
		rel := br.Pkg.SourceFor(e.Path, docSrcs)

		// the source must stay within the repo and the destination within the
		// staging dir, whatever the control files said
		src, err := confine(br.Pkg.Dir(), rel)
		if err != nil {
			return fmt.Errorf("refusing to stage %s: %s", e.Path, err)
		}

		dst, err := confine(br.Tmp, e.Path)
		if err != nil {
			return fmt.Errorf("refusing to stage %s: %s", e.Path, err)
		}

		fi, err := os.Lstat(src)
		switch {
		case os.IsNotExist(err):
			// already warned about by VerifyManifest in insecure mode; skip
			br.dbg.File("SKIPPED", "", e.Path, "not in the repo")
			skipped++
			continue
		case err != nil:
			return fmt.Errorf("failed to stage %s: %s", e.Path, err)
		}

		// CopyFile would follow a symlink and package whatever it resolves to,
		// which may be outside the repo entirely.  `ian add` never registers
		// one, so a symlink here came from a hand edited manifest.
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("refusing to stage %s: %s is not a regular file", e.Path, rel)
		}

		if err := file.CopyFile(src, dst); err != nil {
			return fmt.Errorf("failed to stage %s: %s", e.Path, err)
		}

		// a doc file is copied from elsewhere in the repo, so name both ends
		shown := e.Path
		if rel != e.Path {
			shown = fmt.Sprintf("%s -> %s", rel, e.Path)
		}
		br.dbg.File("staged", "", shown, "")
		staged++
	}

	// stage the DEBIAN control files, including the md5sums manifest itself,
	// which is copied in verbatim so the package ships exactly the sums that
	// were committed and debsums works normally
	for _, fpath := range br.Pkg.CtrlFiles() {
		dst := filepath.Join(br.Tmp, "DEBIAN", filepath.Base(fpath))
		if err := file.CopyFile(fpath, dst); err != nil {
			return fmt.Errorf("failed to stage control file %s: %s", fpath, err)
		}
		br.dbg.File("control", "", filepath.Join("DEBIAN", filepath.Base(fpath)), "")
	}

	br.dbg.Summary("staged %d file(s), skipped %d", staged, skipped)

	// the sums that will be shipped, printed by default so a build leaves a
	// record of what went into the package
	if !br.Quiet {
		m.Write(os.Stderr)
	}

	return nil
}
