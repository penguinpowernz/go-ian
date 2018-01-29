package str

import (
	"os/exec"
	"strings"
)

// CommandString gives the string representation of the given command
func CommandString(cmd *exec.Cmd) string {
	return strings.Join(cmd.Args, " ")
}
