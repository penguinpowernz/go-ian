package ian

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/penguinpowernz/go-ian/util/str"
)

// Ignored will return the ignore patterns from the .ianignore file
func Ignored(dir string) ([]string, error) {
	ign := []string{}
	data, err := ioutil.ReadFile(filepath.Join(dir, ".ianignore"))
	if err != nil {
		return ign, err
	}

	ign = str.CleanStrings(str.Lines(string(data)))
	return ign, nil
}

// IgnoreList will use the ignore file from the package to generate
// a list of ignored file patterns.  If there is no ignore file then
// an empty slice is returned
func (p *Pkg) IgnoreList() []string {
	data, err := ioutil.ReadFile(p.IgnoreFile())
	if err != nil {
		return []string{}
	}

	return str.CleanStrings(str.Lines(string(data)))
}

// IgnoreFile returns the path to the packages ignore file
func (p *Pkg) IgnoreFile() string {
	return p.Dir(".ianignore")
}

// Excludes provides things in the repo to be excluded from the package
func (p *Pkg) Excludes() []string {
	exc := p.IgnoreList()
	exc = append(exc, []string{
		".git", "pkg", ".gitignore", ".ianpush", ".ianignore", ".gitkeep",
	}...)

	return exc
}

// ListFiles returns the list of files that would be included in the package,
// using rsync --list-only with the same exclude rules as the actual build.
func (p *Pkg) ListFiles() ([]string, error) {
	args := []string{"-rav", "--list-only"}
	for _, s := range p.Excludes() {
		if s == "" {
			continue
		}
		args = append(args, fmt.Sprintf("--exclude=%s", s))
	}
	args = append(args, p.Dir()+"/")

	var out bytes.Buffer
	cmd := exec.Command("/usr/bin/rsync", args...)
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to list files: %s", err)
	}

	var files []string
	for _, line := range strings.Split(out.String(), "\n") {
		// rsync --list-only format: "perms size date time filename"
		// skip summary/header lines which don't start with a permission char
		fields := strings.Fields(line)
		if len(fields) < 5 || (fields[0][0] != '-' && fields[0][0] != 'd') {
			continue
		}
		name := fields[4]
		if name == "." {
			continue
		}
		files = append(files, name)
	}
	return files, nil
}
