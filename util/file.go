package util

import (
	"os"
	"os/exec"
	"path/filepath"
)

// PathExists tells you if a path exists
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// FindExec will find the executable with the given name and return
// the absolute path.  If not found, it will return a false.
//
// This resolves the name against PATH in process rather than shelling out to
// which, which means one less external binary on the path and no mangling of
// paths that contain spaces.
func FindExec(binary string) (string, bool) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", false
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}

	return abs, true
}
