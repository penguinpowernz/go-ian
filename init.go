package ian

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/penguinpowernz/go-ian/util"

	"github.com/penguinpowernz/go-ian/debian/control"
)

var blankPostinst = []byte(`
#!/bin/bash

exit 0;
`)

// IsInitialized determines if the directory is already initialized
func IsInitialized(dir string) bool {
	_, err := os.Stat(ControlDir(dir))
	return err == nil
}

// Init will initialize the given directory to be a debian package
func Init(dir string) error {
	initControlDir(dir)
	initControlFile(dir)
	initPostinst(dir)

	return nil
}

func initControlDir(dir string) error {
	exists, err := util.PathExists(dir)
	if err != nil {
		return err
	}

	if !exists {
		err := os.MkdirAll(ControlDir(dir), 0755)
		if err != nil {
			return err
		}
	}

	return nil
}

func initControlFile(dir string) error {
	ctrlFile := ControlFile(dir)
	exists, err := util.PathExists(ctrlFile)
	if err != nil {
		return err
	}

	if !exists {
		ctrl := control.Default(filepath.Base(dir))
		data := []byte(ctrl.String())
		if err = ioutil.WriteFile(ctrlFile, data, 0755); err != nil {
			return err
		}
	}

	return nil
}

func initPostinst(dir string) error {
	piFile := filepath.Join(ControlDir(dir), "postinst")
	exists, err := util.PathExists(piFile)
	if err != nil {
		return err
	}

	if !exists {
		if err = ioutil.WriteFile(piFile, blankPostinst, 0755); err != nil {
			return err
		}
	}

	return nil
}
