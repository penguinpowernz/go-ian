package ian

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/penguinpowernz/go-ian/debian/control"
)

// Package will create a debian package from the given control file and directory. It does this by
// using rsync to copy the repo to a temp dir, excluded unwanted files and moving any files in the root
// of the package to a /usr/share/doc folder.  Then it calculates the package size, file checksums and
// calls dpkg-deb to build the package.  The path to the package and an error (if any) is returned.
func Package(ctrl control.Control, dir string, pkgdest string) (string, error) {
	tmp, err := ioutil.TempDir("/tmp", "go-ian")
	if err != nil {
		return "", fmt.Errorf("couldn't make tmp dir: %s", err)
	}

	defer os.RemoveAll(tmp)

	excludes := PackageExclusions(dir)

	err = stage(dir, tmp, excludes)
	if err != nil {
		return "", fmt.Errorf("failed to stage: %s", err)
	}

	rootFiles, err := findRootFiles(tmp)
	if err != nil {
		return "", fmt.Errorf("failed to find root files: %s", err)
	}

	if err = moveRootFiles(tmp, ctrl.Name, rootFiles); err != nil {
		return "", fmt.Errorf("failed to move root files: %s", err)
	}

	sums := filepath.Join(ControlDir(dir), "md5sums")
	if err = GenerateMD5SUM(tmp, sums); err != nil {
		return "", fmt.Errorf("failed to generate md5sums: %s", err)
	}

	pkgName, err := mkPkgName(ctrl, dir, pkgdest)
	if err != nil {
		return "", fmt.Errorf("failed to make package name: %s", err)
	}

	sizeK, err := CalculateSize(dir, excludes)
	if err != nil {
		return "", fmt.Errorf("failed to calculate package size: %s", err)
	}

	ctrl.Size = sizeK
	ctrl.WriteFile(ControlFile(dir))

	err = build(tmp, pkgName)
	if err != nil {
		return "", fmt.Errorf("failed to build: %s", err)
	}

	return pkgName, nil
}

// PackageExclusions provides things in the repo to be excluded from the package
func PackageExclusions(dir string) []string {
	return []string{
		".git", "pkg", ".gitignore", ".ianpush", ".ianignore",
	}
}

func stage(dir, tmp string, excludes []string) error {
	args := []string{"-rav"}
	for _, s := range excludes {
		args = append(args, fmt.Sprintf("--exclude=%s", s))
	}
	args = append(args, dir+"/", tmp)

	cmd := exec.Command("/usr/bin/rsync", args...)
	// cmd.Stderr = os.Stderr
	// cmd.Stdout = os.Stderr
	err := cmd.Run()
	return err
}

func build(tmp, name string) error {
	cmd := exec.Command("/usr/bin/fakeroot", "dpkg-deb", "-b", tmp, name)
	// cmd.Stderr = os.Stderr
	// cmd.Stdout = os.Stderr
	return cmd.Run()
}

func findRootFiles(dir string) ([]string, error) {
	files := []string{}
	list, err := ioutil.ReadDir(dir)
	if err != nil {
		return files, err
	}

	for _, fi := range list {
		if fi.IsDir() {
			continue
		}

		files = append(files, fi.Name())
	}

	return files, nil
}

func moveRootFiles(dir, pkg string, files []string) error {
	if len(files) == 0 {
		return nil
	}

	docpath := filepath.Join(dir, "usr", "share", "doc", pkg)
	if err := os.MkdirAll(docpath, 0755); err != nil {
		return err
	}

	for _, oldfn := range files {
		newfn := filepath.Join(docpath, filepath.Base(oldfn))
		err := os.Rename(oldfn, newfn)
		if err != nil {
			return err
		}
	}

	return nil
}

func mkPkgName(ctrl control.Control, dir string, pkgdest string) (string, error) {
	if pkgdest == "" {
		pkgdest = filepath.Join(dir, "pkg")
	}

	err := os.MkdirAll(pkgdest, 0755)
	if err != nil {
		return "", err
	}

	return filepath.Join(pkgdest, ctrl.Filename()), nil
}

// CalculateSize of a directory using du, excluding any given paths
func CalculateSize(dir string, excludes []string) (string, error) {
	args := []string{"-sk", dir}
	for _, s := range excludes {
		args = append(args, fmt.Sprintf("--exclude=\"%s\"", s))
	}

	cmd := exec.Command("/usr/bin/du", args...)
	data, err := cmd.Output()
	if err != nil {
		return "", nil
	}

	sizeK := strings.Split(string(data), "\t")[0]
	return sizeK, nil
}

// GenerateMD5SUM of the given directories files and save the output to the given
// outfile path.  Uses find, xargs and md5sum and recurses the entire directory.
func GenerateMD5SUM(dir, outfile string) error {
	find := exec.Command("/usr/bin/find", dir, "-type", "f")
	xargs := exec.Command("/usr/bin/xargs", "md5sum")

	r, w := io.Pipe()
	find.Stdout = w
	xargs.Stdin = r

	var buf bytes.Buffer
	xargs.Stdout = &buf

	if err := find.Start(); err != nil {
		return err
	}

	if err := xargs.Start(); err != nil {
		return err
	}

	if err := find.Wait(); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	if err := xargs.Wait(); err != nil {
		return err
	}

	data := []byte(strings.Replace(string(buf.Bytes()), dir+"/", "", -1))
	return ioutil.WriteFile(outfile, data, 0755)
}
