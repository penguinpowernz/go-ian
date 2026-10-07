package git

import (
	"os/exec"
	"strings"
)

// Describe runs `git describe` in the given directory to derive a version
// string from the repo tags.  The --always flag means a repo with no tags
// still yields the abbreviated commit hash, and --dirty appends -dirty when
// the working tree has uncommitted changes.
func Describe(dir string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}
