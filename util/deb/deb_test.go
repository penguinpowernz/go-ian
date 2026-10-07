package deb

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// stage writes a minimal staged package tree and returns its path
func stage(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	mkdir(t, filepath.Join(dir, "DEBIAN"))
	mkdir(t, filepath.Join(dir, "usr", "bin"))
	mkdir(t, filepath.Join(dir, "etc", "app"))

	write(t, filepath.Join(dir, "DEBIAN", "control"),
		"Package: app\nVersion: 1.0\nArchitecture: all\nMaintainer: t <t@e.com>\nDescription: test\n", 0644)
	write(t, filepath.Join(dir, "DEBIAN", "md5sums"), "abc  usr/bin/app\n", 0644)
	// staged non-executable, to prove the writer forces 0755 on it
	write(t, filepath.Join(dir, "DEBIAN", "postinst"), "#!/bin/sh\nexit 0\n", 0644)
	write(t, filepath.Join(dir, "usr", "bin", "app"), "#!/bin/sh\necho hi\n", 0755)
	write(t, filepath.Join(dir, "etc", "app", "app.conf"), "k = v\n", 0644)

	return dir
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	// WriteFile is subject to umask, so set the mode explicitly
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func build(t *testing.T, dir string) string {
	t.Helper()

	out := filepath.Join(t.TempDir(), "app_1.0_all.deb")
	if err := Build(dir, out, BuildOpts{}); err != nil {
		t.Fatalf("Build: %s", err)
	}

	return out
}

// TestBuildMemberOrder checks the three members a .deb must have, in order.
// dpkg requires debian-binary first and will reject an archive without it.
func TestBuildMemberOrder(t *testing.T) {
	f, err := os.Open(build(t, stage(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	members, err := readAr(f)
	if err != nil {
		t.Fatalf("readAr: %s", err)
	}

	want := []string{"debian-binary", "control.tar.gz", "data.tar.gz"}
	if len(members) != len(want) {
		t.Fatalf("got %d members, want %d", len(members), len(want))
	}

	for i, name := range want {
		if members[i].Name != name {
			t.Errorf("member %d is %q, want %q", i, members[i].Name, name)
		}
	}

	if got := string(members[0].Body); got != debianBinary {
		t.Errorf("debian-binary is %q, want %q", got, debianBinary)
	}
}

// TestRoundTrip builds a package and reads it back, checking the payload
// survives and that DEBIAN is not packaged into the data tarball
func TestRoundTrip(t *testing.T) {
	path := build(t, stage(t))

	data := t.TempDir()
	if err := ExtractData(path, data); err != nil {
		t.Fatalf("ExtractData: %s", err)
	}

	body, err := os.ReadFile(filepath.Join(data, "usr", "bin", "app"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "#!/bin/sh\necho hi\n" {
		t.Errorf("payload came back as %q", body)
	}

	// the control files belong in control.tar.gz only
	if _, err := os.Stat(filepath.Join(data, "DEBIAN")); !os.IsNotExist(err) {
		t.Error("DEBIAN was packaged into the data tarball")
	}

	ctrl := t.TempDir()
	if err := ExtractControl(path, ctrl); err != nil {
		t.Fatalf("ExtractControl: %s", err)
	}

	for _, name := range []string{"control", "md5sums", "postinst"} {
		if _, err := os.Stat(filepath.Join(ctrl, name)); err != nil {
			t.Errorf("control file %s missing: %s", name, err)
		}
	}
}

// TestModes checks that maintainer scripts come out executable whatever their
// staged mode, that other control files do not, and that a payload file keeps
// its own permissions
func TestModes(t *testing.T) {
	path := build(t, stage(t))

	ctrl := t.TempDir()
	if err := ExtractControl(path, ctrl); err != nil {
		t.Fatal(err)
	}

	// staged 0644, but dpkg has to be able to execute it
	if mode := modeOf(t, filepath.Join(ctrl, "postinst")); mode != 0755 {
		t.Errorf("postinst is %o, want 755", mode)
	}

	if mode := modeOf(t, filepath.Join(ctrl, "control")); mode != 0644 {
		t.Errorf("control is %o, want 644", mode)
	}

	data := t.TempDir()
	if err := ExtractData(path, data); err != nil {
		t.Fatal(err)
	}

	if mode := modeOf(t, filepath.Join(data, "usr", "bin", "app")); mode != 0755 {
		t.Errorf("packaged binary is %o, want 755", mode)
	}

	if mode := modeOf(t, filepath.Join(data, "etc", "app", "app.conf")); mode != 0644 {
		t.Errorf("packaged conf is %o, want 644", mode)
	}
}

func modeOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode().Perm()
}

// TestRootOwned is the fakeroot replacement: everything in both tarballs must
// be owned by uid 0 / gid 0 even though the staged files are owned by whoever
// ran the build
func TestRootOwned(t *testing.T) {
	f, err := os.Open(build(t, stage(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	members, err := readAr(f)
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range members[1:] {
		r, err := decompress(m)
		if err != nil {
			t.Fatal(err)
		}

		for _, hdr := range headers(t, r) {
			if hdr.Uid != 0 || hdr.Gid != 0 {
				t.Errorf("%s/%s is owned by %d:%d, want 0:0", m.Name, hdr.Name, hdr.Uid, hdr.Gid)
			}
			if hdr.Uname != "root" || hdr.Gname != "root" {
				t.Errorf("%s/%s is owned by %s:%s, want root:root", m.Name, hdr.Name, hdr.Uname, hdr.Gname)
			}
		}
	}
}

// TestReproducible checks that the same staged tree gives byte identical
// output, which is what lets a package be built twice and compared
func TestReproducible(t *testing.T) {
	dir := stage(t)

	first, err := os.ReadFile(build(t, dir))
	if err != nil {
		t.Fatal(err)
	}

	// touch the staged files, so anything reading their mtimes would differ
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "usr", "bin", "app"), future, future); err != nil {
		t.Fatal(err)
	}

	second, err := os.ReadFile(build(t, dir))
	if err != nil {
		t.Fatal(err)
	}

	if string(first) != string(second) {
		t.Error("two builds of the same tree produced different bytes")
	}
}

// TestSourceDateEpoch checks the reproducible-builds environment variable is
// honoured, so a caller can pin the timestamp the conventional way
func TestSourceDateEpoch(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")

	want := time.Unix(1700000000, 0).UTC()
	if got := (BuildOpts{}).modTime(); !got.Equal(want) {
		t.Errorf("modTime is %s, want %s", got, want)
	}

	// an explicit option still wins
	explicit := time.Unix(42, 0).UTC()
	if got := (BuildOpts{ModTime: explicit}).modTime(); !got.Equal(explicit) {
		t.Errorf("modTime is %s, want the explicit %s", got, explicit)
	}

	// garbage in the environment falls back rather than failing the build
	t.Setenv("SOURCE_DATE_EPOCH", "not-a-number")
	if got := (BuildOpts{}).modTime(); !got.Equal(defaultModTime) {
		t.Errorf("modTime is %s, want the default %s", got, defaultModTime)
	}
}

// TestBuildRejectsBadStaging checks a staged tree without the control files is
// refused rather than producing a package dpkg cannot install
func TestBuildRejectsBadStaging(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.deb")

	if err := Build(t.TempDir(), out, BuildOpts{}); err == nil {
		t.Error("built a package from a tree with no DEBIAN dir")
	}

	// a DEBIAN dir but no control file in it
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "DEBIAN"))
	if err := Build(dir, out, BuildOpts{}); err == nil {
		t.Error("built a package with no control file")
	}
}

// TestBuildLeavesNoPartialPackage checks a failed build does not leave
// something behind that looks like a finished package
func TestBuildLeavesNoPartialPackage(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "x.deb")

	if err := Build(t.TempDir(), out, BuildOpts{}); err == nil {
		t.Fatal("expected the build to fail")
	}

	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Error("a failed build left a package behind")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a failed build left %d file(s) behind", len(entries))
	}
}

// TestUntarRefusesTraversal checks that a package naming a path outside the
// extraction dir cannot write there, so verifying a hostile .deb cannot
// scribble over the filesystem.
//
// A traversing path is rejected outright.  An absolute path is instead
// re-rooted inside the extraction dir, which is what dpkg does with the
// absolute paths every package legitimately contains.
func TestUntarRefusesTraversal(t *testing.T) {
	rejected := []string{"../escaped", "./../escaped", "a/../../escaped"}

	for _, name := range rejected {
		// a dir inside a dir, so an escape by one level is still observable
		parent := t.TempDir()
		dir := filepath.Join(parent, "extract")

		if err := untar(tarWith(t, name), dir); err == nil {
			t.Errorf("%q: extracted without an error", name)
		}

		if _, err := os.Stat(filepath.Join(parent, "escaped")); !os.IsNotExist(err) {
			t.Errorf("%q escaped the extraction dir", name)
		}
	}

	// an absolute path lands under the extraction dir, not at the real path
	parent := t.TempDir()
	dir := filepath.Join(parent, "extract")

	if err := untar(tarWith(t, "/abs/escaped"), dir); err != nil {
		t.Fatalf("absolute path: %s", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "abs", "escaped")); err != nil {
		t.Errorf("absolute path was not re-rooted into the extraction dir: %s", err)
	}
}

// TestArRejectsBadMagic checks a file that is not an ar archive is reported as
// such rather than being read as garbage
func TestArRejectsBadMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notadeb")
	write(t, path, "this is not an ar archive at all", 0644)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := readAr(f); err == nil {
		t.Error("read a non-ar file as an ar archive")
	}
}

// headers reads every tar header from r
func headers(t *testing.T, r io.Reader) []*tar.Header {
	t.Helper()

	var hdrs []*tar.Header

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return hdrs
		}
		if err != nil {
			t.Fatal(err)
		}
		hdrs = append(hdrs, hdr)
	}
}

// tarWith returns a tar stream containing a single regular file at the given
// path, for testing what the extractor does with a hostile entry name
func tarWith(t *testing.T, name string) io.Reader {
	t.Helper()

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)
	body := []byte("payload")

	err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     int64(len(body)),
		Format:   tar.FormatGNU,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	return &buf
}
