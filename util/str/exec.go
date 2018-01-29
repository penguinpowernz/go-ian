package str

import (
	"os/exec"
	"strings"
)

func CommandString(cmd *exec.Cmd) string {
	return strings.Join(append([]string{cmd.Path}, cmd.Args...), " ")
}
