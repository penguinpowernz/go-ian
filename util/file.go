package util

import (
	"os"
	"os/exec"
	"strings"
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
// the absolute path.  If not found, it will return a false
func FindExec(binary string) (string, bool) {
	cmd := exec.Command("/bin/which", binary)
	data, err := cmd.Output()
	if err != nil {
		return "", false
	}

	return strings.TrimSpace(strings.Split(string(data), " ")[0]), true
}
