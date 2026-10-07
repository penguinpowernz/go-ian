package file

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/yargevad/filepathx"
)

// Exists returns true if the given path exists.  An error other than "not
// exists" (a permission denied on a parent directory, say) means the path
// could not be resolved either way, which is reported as not existing rather
// than as a file that is there but unreadable.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

// DirSize calculates the apparent size of the directory tree in bytes.  It
// walks the tree in process rather than shelling out to du, both so a build
// does not depend on an external binary and because du -b is GNU only.
//
// Only regular files count towards the total.  Symlinks are counted by the
// size of the link target path rather than followed, so a link cannot inflate
// the total with the size of whatever it points at.
func DirSize(dir string) (int, error) {
	var total int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		switch {
		case info.Mode().IsRegular():
			total += info.Size()
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			total += int64(len(target))
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return int(total), nil
}

// CopyFile copies the file at src to dst, creating any parent directories of
// dst as needed and preserving the source file's mode
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	fi, err := in.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Chmod(fi.Mode())
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
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	return f.Close()
}

// Glob is a lazier form of filepath.Glob in that it will use filepath.Join on the provided arguments
// before returning the resulting globbed files, ignoring the error that filepath.Glob returns
func Glob(path ...string) []string {
	f, _ := filepathx.Glob(filepath.Join(path...))
	return f
}
