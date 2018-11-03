package ian

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/penguinpowernz/go-ian/util"
	"github.com/penguinpowernz/go-ian/util/tell"
)

type target struct {
	name string
	cmd  *exec.Cmd
	send func(string) error
}

func (t *target) Push() error {
	return t.cmd.Run()
}

func parseTargets(data []byte, pkg string) (tgts []*target) {
	text := string(data)

	if strings.TrimSpace(text) == "" {
		return
	}

	lines := strings.Split(text, "\n")

	if len(lines) == 0 {
		return
	}

	// check if the lines are named
	re := regexp.MustCompile(`[0-9a-zA-Z_\-\*]*:`)
	var err error
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}

		t := new(target)
		t.name = "default"

		cmdString := l
		if re.MatchString(l) {
			bits := strings.Split(cmdString, ":")
			t.name = bits[0]
			cmdString = strings.Join(bits[1:], "")
			t.cmd, err = makeCmd(cmdString, pkg)
			if err != nil {
				tell.IfErrorf(err, "failed to parse command from %s", cmdString)
				continue
			}

		} else {
			t.cmd, err = makeCmd(cmdString, pkg)
			if err != nil {
				tell.IfErrorf(err, "failed to parse command from %s", cmdString)
				continue
			}

		}

		tgts = append(tgts, t)
	}

	return
}

func selectTargets(tgts []*target, slctr string) (slctd []*target) {
	for _, t := range tgts {
		if yes, _ := filepath.Match(slctr, t.name); yes {
			slctd = append(slctd, t)
		}
	}
	return
}

func makeCmd(cmdString string, pkg string) (*exec.Cmd, error) {
	cmdString = strings.TrimSpace(cmdString)
	bits := strings.Split(cmdString, " ")
	xctbl := bits[0]
	args := bits[1:]
	xctbl, found := util.FindExec(xctbl)
	if !found {
		return nil, fmt.Errorf("couldn't find location of %s", bits[0])
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

	return cmd, nil
}

// Push will run the given pkg name against the commands found in
// pushFile given.  This is meant to be used to push the packages
// to repositories
func Push(pushFile, pkg string, slctr string) error {
	if slctr == "" {
		slctr = "default"
	}

	_, err := os.Stat(pkg)
	if os.IsNotExist(err) {
		return fmt.Errorf("couldn't find package %s", pkg)
	}

	data, err := ioutil.ReadFile(pushFile)
	if err != nil {
		return fmt.Errorf("couldn't read push file %s: %s", pushFile, err)
	}

	targets := selectTargets(parseTargets(data, pkg), slctr)
	if len(targets) == 0 {
		return fmt.Errorf("no targets to push to")
	}

	succeeded := 0
	attempts := 0
	for _, t := range targets {
		attempts++
		tell.Debugf("pushing to target %s: %s", t.name, strings.Join(t.cmd.Args, " "))
		err = t.Push()
		tell.IfErrorf(err, "running %+v failed", t.cmd.Args)
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
