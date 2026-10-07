package file

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "there"), "hi\n")

	if !Exists(filepath.Join(dir, "there")) {
		t.Error("an existing file reported as missing")
	}
	if !Exists(dir) {
		t.Error("an existing directory reported as missing")
	}
	if Exists(filepath.Join(dir, "nope")) {
		t.Error("a missing file reported as existing")
	}
}

func TestListFilesIn(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a"), "a")
	write(t, filepath.Join(dir, "b"), "b")
	write(t, filepath.Join(dir, "sub", "c"), "c")

	got, err := ListFilesIn(dir)
	if err != nil {
		t.Fatal(err)
	}

	// directories and their contents are left out: the listing is not recursive
	want := []string{filepath.Join(dir, "a"), filepath.Join(dir, "b")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListFilesIn() = %q, want %q", got, want)
	}

	if _, err := ListFilesIn(filepath.Join(dir, "nope")); err == nil {
		t.Error("expected an error for a missing directory")
	}
}

func TestMoveFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a"), "a")
	write(t, filepath.Join(dir, "sub", "b"), "b")

	dest := filepath.Join(dir, "dest", "deeper")
	paths := []string{filepath.Join(dir, "a"), filepath.Join(dir, "sub", "b")}

	if err := MoveFiles(paths, dest); err != nil {
		t.Fatal(err)
	}

	// the destination is created, the files land under their base names and
	// the originals are gone
	for _, name := range []string{"a", "b"} {
		if !Exists(filepath.Join(dest, name)) {
			t.Errorf("%s did not arrive in the destination", name)
		}
	}
	for _, p := range paths {
		if Exists(p) {
			t.Errorf("%s was left behind", p)
		}
	}
}

func TestMoveFilesNothingToMove(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "dest")

	// with no files to move the destination is not created either
	if err := MoveFiles(nil, dest); err != nil {
		t.Fatal(err)
	}
	if Exists(dest) {
		t.Error("destination created for an empty move")
	}
}

func TestMoveFilesMissingSource(t *testing.T) {
	dir := t.TempDir()
	err := MoveFiles([]string{filepath.Join(dir, "nope")}, filepath.Join(dir, "dest"))
	if err == nil {
		t.Error("expected an error moving a file that isn't there")
	}
}

func TestDirSize(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a"), "12345")       // 5
	write(t, filepath.Join(dir, "sub", "b"), "1234") // 4

	got, err := DirSize(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != 9 {
		t.Errorf("DirSize() = %d, want 9", got)
	}
}

func TestDirSizeCountsSymlinkTargetPath(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "big"), "0123456789")

	// the link is counted by the length of its target path, not by following
	// it, so a link to a large file cannot inflate the total
	if err := os.Symlink(filepath.Join(dir, "big"), filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %s", err)
	}

	got, err := DirSize(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := 10 + len(filepath.Join(dir, "big"))
	if got != want {
		t.Errorf("DirSize() = %d, want %d", got, want)
	}
}

func TestDirSizeMissingDir(t *testing.T) {
	if _, err := DirSize(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected an error for a missing directory")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	write(t, src, "contents\n")
	if err := os.Chmod(src, 0751); err != nil {
		t.Fatal(err)
	}

	// the destination's parents do not exist yet
	dst := filepath.Join(dir, "a", "b", "dst")
	if err := CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "contents\n" {
		t.Errorf("copied contents = %q", data)
	}

	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0751 {
		t.Errorf("mode = %o, want 751", fi.Mode().Perm())
	}
}

func TestCopyFileOverwrites(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	write(t, src, "new\n")
	write(t, dst, "much longer old contents\n")

	if err := CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}

	// the old contents are truncated away rather than partly overwritten
	data, err := ioutil.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new\n" {
		t.Errorf("copied contents = %q, want %q", data, "new\n")
	}
}

func TestCopyFileMissingSource(t *testing.T) {
	dir := t.TempDir()
	if err := CopyFile(filepath.Join(dir, "nope"), filepath.Join(dir, "dst")); err == nil {
		t.Error("expected an error copying a file that isn't there")
	}
}

func TestCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "file")
	if err := Create(path, []byte("data")); err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "data" {
		t.Errorf("contents = %q", data)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0755 {
		t.Errorf("mode = %o, want 755", fi.Mode().Perm())
	}
}

func TestEmptyBashScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "postinst")
	if err := EmptyBashScript(path); err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// a maintainer script is no use without a shebang, and must exit cleanly
	if got := string(data); len(got) < 2 || got[:2] != "#!" {
		t.Errorf("script does not start with a shebang: %q", got)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0100 == 0 {
		t.Errorf("script is not executable: mode %o", fi.Mode().Perm())
	}
}

func TestEmptyDotFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".ianpush")
	if err := EmptyDotFile(path); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != 0 {
		t.Errorf("size = %d, want 0", fi.Size())
	}

	// creating it again must not clobber what is already there
	write(t, path, "keep me\n")
	if err := EmptyDotFile(path); err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep me\n" {
		t.Errorf("existing contents clobbered: %q", data)
	}
}

func TestGlob(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.txt"), "a")
	write(t, filepath.Join(dir, "b.txt"), "b")
	write(t, filepath.Join(dir, "c.md"), "c")

	got := Glob(dir, "*.txt")
	sort.Strings(got)
	want := []string{filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Glob() = %q, want %q", got, want)
	}

	// a pattern matching nothing gives an empty list rather than an error
	if got := Glob(dir, "*.nope"); len(got) != 0 {
		t.Errorf("Glob() = %q, want nothing", got)
	}
}

func TestGlobRecursive(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.txt"), "a")
	write(t, filepath.Join(dir, "sub", "deep", "b.txt"), "b")

	// filepathx gives ** its recursive meaning, unlike filepath.Glob
	got := Glob(dir, "**", "*.txt")
	if len(got) == 0 {
		t.Fatal("Glob() with ** matched nothing")
	}

	var found bool
	for _, p := range got {
		if p == filepath.Join(dir, "sub", "deep", "b.txt") {
			found = true
		}
	}
	if !found {
		t.Errorf("Glob() = %q, missing the nested file", got)
	}
}
