package ian

import (
	"io/ioutil"
	"path/filepath"

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

func (p *Pkg) IgnoreList() []string {
	data, err := ioutil.ReadFile(p.IgnoreFile())
	if err != nil {
		return []string{}
	}

	return str.CleanStrings(str.Lines(string(data)))
}

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
