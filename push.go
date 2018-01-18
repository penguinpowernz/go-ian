package ian

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"bitbucket.org/autogrowsystems/go-sdk/util"
	"github.com/AutogrowSystems/go-intelli/util/tell"
)

func Push(pushFile, pkg string) error {
	_, err := os.Stat(pkg)
	if os.IsNotExist(err) {
		return fmt.Errorf("couldn't find package %s", pkg)
	}

	data, err := ioutil.ReadFile(pushFile)
	if err != nil {
		return fmt.Errorf("couldn't read push file %s: %s", pushFile, err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		bits := strings.Split(line, " ")
		bits = append(bits, pkg)
		xctbl := bits[0]
		args := bits[1:]
		xctbl, found := util.FindExec(xctbl)
		if !found {
			tell.Errorf("couldn't find location of %s", xctbl)
			continue
		}

		cmd := exec.Command(xctbl, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		tell.IfErrorf(err, "running %s %+v failed", xctbl, args)
	}

	return nil
}
