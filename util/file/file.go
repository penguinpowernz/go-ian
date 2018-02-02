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

// Exists returns true if the given path exists
func Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// ListFilesIn will give a list of all the files in the root of the given
// directory (i.e. not recursive)
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

		files = append(files, filepath.Join(dir, fi.Name()))
	}

	return files, nil
}

// MoveFiles will move all the files in the given slice to the given
// destination.  It will create the destination if need be
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
			return fmt.Errorf("couldn't move %s to new location %s: %s", oldfn, newfn, err)
		}

		if !Exists(newfn) {
			return fmt.Errorf("file %s didn't move to new location %s", oldfn, newfn)
		}

		if Exists(oldfn) {
			if err := os.Remove(oldfn); err != nil {
				return fmt.Errorf("couldn't remove old file %s: %s", oldfn, err)
			}
		}
	}

	return nil

}

// DirSize uses du to calculate the directory size in bytes
func DirSize(dir string, excludes []string) (int, error) {
	args := []string{"-bs", dir}
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

// Create is a shortcut to create a file with perms of 755 and
// the given contents
func Create(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, 0755)
}

// EmptyBashScript creates an empty bashscript at the given filepath
func EmptyBashScript(fn string) error {
	return Create(fn, []byte(`#!/bin/bash

# your code goes here

exit 0;
`))
}

// EmptyDotFile creates an empty dotfile at the given filepath
func EmptyDotFile(path string) error {
	_, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0666)
	return err
}
