package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// TestExtractMissingMember checks a .deb that does not hold the member being
// asked for is reported as such, rather than silently extracting nothing
func TestExtractMissingMember(t *testing.T) {
	path := filepath.Join(t.TempDir(), "odd.deb")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = writeAr(f, []arMember{{Name: "debian-binary", Body: []byte("2.0\n")}})
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	for _, fn := range []func(string, string) error{ExtractControl, ExtractData} {
		if err := fn(path, t.TempDir()); err == nil {
			t.Error("extracting a member that isn't there returned no error")
		}
	}
}

func TestExtractMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.deb")

	if err := ExtractData(missing, t.TempDir()); err == nil {
		t.Error("extracting a file that isn't there returned no error")
	}
	if err := ExtractControl(missing, t.TempDir()); err == nil {
		t.Error("extracting a file that isn't there returned no error")
	}
}

func TestExtractNotADeb(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notadeb")
	write(t, path, "this is not an ar archive at all", 0644)

	if err := ExtractData(path, t.TempDir()); err == nil {
		t.Error("extracting a file that isn't a .deb returned no error")
	}
}

func TestDecompress(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	t.Run("gzip", func(t *testing.T) {
		r, err := decompress(arMember{Name: "data.tar.gz", Body: gz.Bytes()})
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "payload" {
			t.Errorf("decompressed to %q", body)
		}
	})

	t.Run("uncompressed", func(t *testing.T) {
		r, err := decompress(arMember{Name: "data.tar", Body: []byte("payload")})
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "payload" {
			t.Errorf("read back as %q", body)
		}
	})

	// xz, zstd and bzip2 are legal in a .deb but need a dependency to read,
	// so say so rather than failing obscurely further down
	t.Run("unsupported compression is named", func(t *testing.T) {
		_, err := decompress(arMember{Name: "data.tar.xz", Body: []byte("x")})
		if err == nil {
			t.Fatal("no error for an unsupported compression format")
		}
		if !strings.Contains(err.Error(), "data.tar.xz") {
			t.Errorf("error does not name the member: %s", err)
		}
	})

	t.Run("corrupt gzip errors", func(t *testing.T) {
		if _, err := decompress(arMember{Name: "data.tar.gz", Body: []byte("not gzip")}); err == nil {
			t.Error("no error for a corrupt gzip member")
		}
	})
}

// TestUntarEntryTypes checks what untar does with each entry type a .deb may
// hold: directories and files are created, symlinks are recreated without
// being followed, and device nodes are skipped rather than attempted
func TestUntarEntryTypes(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	hdrs := []*tar.Header{
		{Name: "./usr/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "./usr/bin/app", Typeflag: tar.TypeReg, Mode: 0755, Size: 4},
		{Name: "./usr/bin/link", Typeflag: tar.TypeSymlink, Mode: 0777, Linkname: "app"},
		{Name: "./dev/null", Typeflag: tar.TypeChar, Mode: 0666, Devmajor: 1, Devminor: 3},
		{Name: "./run/fifo", Typeflag: tar.TypeFifo, Mode: 0666},
	}

	for _, h := range hdrs {
		h.Format = tar.FormatGNU
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte("hi!\n")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := untar(&buf, dir); err != nil {
		t.Fatalf("untar: %s", err)
	}

	if fi, err := os.Stat(filepath.Join(dir, "usr")); err != nil || !fi.IsDir() {
		t.Errorf("directory entry not extracted: %v %v", fi, err)
	}

	body, err := os.ReadFile(filepath.Join(dir, "usr", "bin", "app"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hi!\n" {
		t.Errorf("file came back as %q", body)
	}

	// the link is recreated but never resolved, so its target need not exist
	target, err := os.Readlink(filepath.Join(dir, "usr", "bin", "link"))
	if err != nil {
		t.Errorf("symlink not extracted: %s", err)
	} else if target != "app" {
		t.Errorf("symlink points at %q, want %q", target, "app")
	}

	// creating these needs privileges and ian never packages them
	for _, skipped := range []string{"dev/null", "run/fifo"} {
		if _, err := os.Lstat(filepath.Join(dir, skipped)); !os.IsNotExist(err) {
			t.Errorf("%s was extracted", skipped)
		}
	}
}

// TestUntarRejectsTruncatedFile checks a stream whose header claims more bytes
// than the body carries is reported rather than yielding a short file
func TestUntarRejectsTruncatedFile(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	body := []byte("short")
	err := tw.WriteHeader(&tar.Header{
		Name:     "./usr/bin/app",
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

	// lop off the body and its padding, leaving the header claiming 5 bytes
	truncated := buf.Bytes()[:512]

	if err := untar(bytes.NewReader(truncated), t.TempDir()); err == nil {
		t.Error("a truncated archive extracted without an error")
	}
}

func TestUntarEmptyAndRootEntriesAreSkipped(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// a .deb's data tarball starts with an entry for its own root
	for _, name := range []string{"./", "/"} {
		err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeDir,
			Mode:     0755,
			Format:   tar.FormatGNU,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), "extract")
	if err := untar(&buf, dir); err != nil {
		t.Fatalf("untar: %s", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("extracted %d entries, want none", len(entries))
	}
}

func TestConfine(t *testing.T) {
	root := "/tmp/root"

	for _, rel := range []string{"usr/bin/app", "usr", "."} {
		if _, err := confine(root, rel); err != nil {
			t.Errorf("confine(%q) errored: %s", rel, err)
		}
	}

	for _, rel := range []string{"../escaped", "usr/../../escaped", ".."} {
		if _, err := confine(root, rel); err == nil {
			t.Errorf("confine(%q) did not error", rel)
		}
	}

	// a directory merely sharing the root's prefix is not inside it
	if _, err := confine("/tmp/root", "../rootkit/x"); err == nil {
		t.Error("confine let a sibling directory through")
	}
}

func TestBuildMissingOutputDir(t *testing.T) {
	out := filepath.Join(t.TempDir(), "nope", "app_1.0_all.deb")

	if err := Build(stage(t), out, BuildOpts{}); err == nil {
		t.Error("building into a directory that isn't there returned no error")
	}
}
