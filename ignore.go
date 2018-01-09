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
