package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestPathExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "there")
	if err := ioutil.WriteFile(file, []byte("hi\n"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		path string
		want bool
	}{
		{"an existing file", file, true},
		{"an existing directory", dir, true},
		{"a missing file", filepath.Join(dir, "nope"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PathExists(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("PathExists(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestFindExec(t *testing.T) {
	path, found := FindExec("sh")
	if !found {
		t.Fatal("sh not found on PATH")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("FindExec() = %q, want an absolute path", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("FindExec() returned a path that doesn't exist: %s", err)
	}
}

func TestFindExecMissing(t *testing.T) {
	if path, found := FindExec("definitely-not-a-real-binary-xyzzy"); found {
		t.Errorf("FindExec() found %q for a binary that doesn't exist", path)
	}
}

func TestFindExecHonoursPath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(bin, "ian-test-bin")
	if err := ioutil.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// a push command names a bare binary, so it has to be resolved against
	// PATH rather than assumed to live anywhere in particular
	t.Setenv("PATH", bin)

	path, found := FindExec("ian-test-bin")
	if !found {
		t.Fatal("ian-test-bin not found on PATH")
	}
	if path != exe {
		t.Errorf("FindExec() = %q, want %q", path, exe)
	}
}

func TestFindExecNonExecutableIsNotFound(t *testing.T) {
	bin := t.TempDir()
	if err := ioutil.WriteFile(filepath.Join(bin, "ian-test-data"), []byte("not a program\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", bin)

	if path, found := FindExec("ian-test-data"); found {
		t.Errorf("FindExec() found %q for a file that isn't executable", path)
	}
}
