package file

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func ListFilesIn(dir string) ([]string, error) {
	files := []string{}
	list, err := ioutil.ReadDir(dir)
	if err != nil {
		return files, err
	}

	for _, fi := range list {
		if fi.IsDir() {
			continue
		}

		files = append(files, fi.Name())
	}

	return files, nil
}

func MoveFiles(paths []string, dest string) error {

	if len(paths) == 0 {
		return nil
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	for _, oldfn := range paths {
		newfn := filepath.Join(dest, filepath.Base(oldfn))
		err := os.Rename(oldfn, newfn)
		if err != nil {
			return err
		}
	}

	return nil

}

// DirSize uses du to calculate the directory size in bytes
func DirSize(dir string, excludes []string) (int, error) {
	args := []string{"-s", dir}
	for _, s := range excludes {
		args = append(args, fmt.Sprintf("--exclude=\"%s\"", s))
	}

	cmd := exec.Command("/usr/bin/du", args...)
	data, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(strings.Split(string(data), "\t")[0])
}

func Create(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, 0755)
}

func EmptyBashScript(fn string) error {
	return Create(fn, []byte(`#!/bin/bash

# your code goes here

exit 0;
`))
}

func EmptyDotFile(path string) error {
	_, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0666)
	return err
}
