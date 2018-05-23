package ian

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/penguinpowernz/go-ian/util"
	"github.com/penguinpowernz/go-ian/util/tell"
)

// Push will run the given pkg name against the commands found in
// pushFile given.  This is meant to be used to push the packages
// to repositories
func Push(pushFile, pkg string) error {
	_, err := os.Stat(pkg)
	if os.IsNotExist(err) {
		return fmt.Errorf("couldn't find package %s", pkg)
	}

	data, err := ioutil.ReadFile(pushFile)
	if err != nil {
		return fmt.Errorf("couldn't read push file %s: %s", pushFile, err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimSpace(string(data)) == "" {
		return fmt.Errorf("no targets to push to")
	}

	succeeded := 0
	attempts := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		attempts++
		bits := strings.Split(line, " ")
		xctbl := bits[0]
		args := bits[1:]
		xctbl, found := util.FindExec(xctbl)
		if !found {
			tell.Errorf("couldn't find location of %s", bits[0])
			continue
		}

		// check for inline string replacement in the args
		inline := false
		for i, arg := range args {
			if strings.Contains(arg, "$PKG") {
				args[i] = strings.Replace(arg, "$PKG", pkg, -1)
				inline = true
			}
		}

		// if none inline, then make it the last argument
		if !inline {
			args = append(args, pkg)
		}

		cmd := exec.Command(xctbl, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		tell.IfErrorf(err, "running %s %+v failed", xctbl, args)
		if err == nil {
			succeeded++
		}
	}

	if attempts != succeeded {
		tell.Errorf("pushed to %d / %d targets", succeeded, attempts)
		return fmt.Errorf("some lines failed to execute")
	}

	tell.Infof("pushed to %d / %d targets", succeeded, attempts)
	return nil
}
