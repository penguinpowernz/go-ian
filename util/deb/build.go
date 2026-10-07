package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// debianBinary is the contents of the debian-binary member, naming the .deb
// format version this writes
const debianBinary = "2.0\n"

// maintainerScripts are the control files dpkg executes, and so the only ones
// written 0755.  Every other control file is data dpkg only reads.
var maintainerScripts = map[string]bool{
	"preinst":  true,
	"postinst": true,
	"prerm":    true,
	"postrm":   true,
	"config":   true,
}

// defaultModTime is stamped on archive entries when BuildOpts.ModTime is not
// set.  It is a fixed, arbitrary point rather than the epoch itself because
// lintian flags a 1970 timestamp as an "ancient file", and rather than the
// build time because that would make the output differ on every run.
//
// Anything fixed keeps the build reproducible; this is simply a fixed value
// that does not trip the warning.  SOURCE_DATE_EPOCH overrides it.
var defaultModTime = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)

// BuildOpts controls how Build writes the package
type BuildOpts struct {
	// ModTime is stamped on every entry in both tarballs.  Stamping one time
	// over the whole archive rather than each file's own mtime is what lets
	// the same staged tree produce the same bytes every time.
	//
	// Zero means SOURCE_DATE_EPOCH if that is set in the environment, and
	// defaultModTime otherwise.
	ModTime time.Time
}

// modTime resolves the timestamp to stamp on the archive entries, honouring
// SOURCE_DATE_EPOCH so a caller can pin the build the conventional way
// (see https://reproducible-builds.org/specs/source-date-epoch/)
func (o BuildOpts) modTime() time.Time {
	if !o.ModTime.IsZero() {
		return o.ModTime
	}

	if s := os.Getenv("SOURCE_DATE_EPOCH"); s != "" {
		if secs, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(secs, 0).UTC()
		}
	}

	return defaultModTime
}

// Build writes the staged directory at dir as a binary .deb at out.
//
// dir is a staged package root: a DEBIAN directory holding the control files,
// alongside the filesystem tree to install.  Everything is written owned by
// uid 0 / gid 0 regardless of who owns the staged files, which is what makes
// fakeroot unnecessary.
//
// The caller is responsible for the contents and permissions of the staged
// tree; Build only normalises what the .deb format itself dictates (the
// ownership, the control file modes, and the directory modes).
func Build(dir, out string, opts BuildOpts) error {
	ctrlDir := filepath.Join(dir, "DEBIAN")
	fi, err := os.Stat(ctrlDir)
	if err != nil {
		return fmt.Errorf("couldn't read the control dir %s: %s", ctrlDir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", ctrlDir)
	}
	if !fileExists(filepath.Join(ctrlDir, "control")) {
		return fmt.Errorf("no control file in %s", ctrlDir)
	}

	modTime := opts.modTime()

	control, err := tarball(ctrlDir, "", modTime, controlMode)
	if err != nil {
		return fmt.Errorf("failed to build control.tar.gz: %s", err)
	}

	// the data tarball is the staged tree with DEBIAN/ left out, pathed from
	// the package root as ./usr/bin/foo and so on
	data, err := tarball(dir, "DEBIAN", modTime, dataMode)
	if err != nil {
		return fmt.Errorf("failed to build data.tar.gz: %s", err)
	}

	// write to a temp file beside the target and rename, so an interrupted
	// build cannot leave a half written .deb that looks finished
	tmp, err := os.CreateTemp(filepath.Dir(out), ".ian-deb-*")
	if err != nil {
		return fmt.Errorf("couldn't create a temp file for %s: %s", out, err)
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	err = writeAr(tmp, []arMember{
		{Name: "debian-binary", Body: []byte(debianBinary)},
		{Name: "control.tar.gz", Body: control},
		{Name: "data.tar.gz", Body: data},
	})
	if err != nil {
		return fmt.Errorf("failed to write %s: %s", out, err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write %s: %s", out, err)
	}

	if err := os.Chmod(tmp.Name(), 0644); err != nil {
		return fmt.Errorf("failed to set the mode on %s: %s", out, err)
	}

	if err := os.Rename(tmp.Name(), out); err != nil {
		return fmt.Errorf("failed to move the package into place at %s: %s", out, err)
	}

	return nil
}

// modeFunc decides the mode written for a staged file, given its path relative
// to the tar root and the mode it has on disk
type modeFunc func(rel string, on os.FileMode) os.FileMode

// controlMode gives the maintainer scripts 0755 and every other control file
// 0644, so a staged file's mode on disk cannot make a control file executable
// that dpkg only ever reads
func controlMode(rel string, on os.FileMode) os.FileMode {
	if maintainerScripts[filepath.Base(rel)] {
		return 0755
	}
	return 0644
}

// dataMode keeps the staged file's own permission bits, since which installed
// files are executable is the package's business, but drops setuid, setgid and
// the sticky bit along with any Go specific mode flags
func dataMode(rel string, on os.FileMode) os.FileMode {
	return on.Perm()
}

// tarball walks root and returns it as a gzipped tar.
//
// skip names a directory at the root to leave out (DEBIAN, when writing the
// data tarball).  mode decides the mode for each file.  Entries are sorted by
// path and written with a fixed uid, gid, owner name and mtime so that the same
// staged tree always produces the same bytes.
func tarball(root, skip string, modTime time.Time, mode modeFunc) ([]byte, error) {
	var paths []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		if skip != "" && (rel == skip || strings.HasPrefix(rel, skip+string(filepath.Separator))) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// a fixed order, so the same tree always gives the same archive
	sort.Strings(paths)

	var buf bytes.Buffer

	// the gzip header carries an mtime and a name of its own, so it is built
	// explicitly rather than with gzip.NewWriter's defaults
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	zw.ModTime = modTime
	zw.OS = 3 // Unix, rather than whatever the host happens to be

	tw := tar.NewWriter(zw)

	// dpkg expects the archive to open with the root directory
	rootHdr := &tar.Header{
		Name:     "./",
		Typeflag: tar.TypeDir,
		Mode:     0755,
		Uid:      0,
		Gid:      0,
		Uname:    "root",
		Gname:    "root",
		ModTime:  modTime,
		Format:   tar.FormatGNU,
	}
	if err := tw.WriteHeader(rootHdr); err != nil {
		return nil, err
	}

	for _, rel := range paths {
		path := filepath.Join(root, rel)

		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}

		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			if link, err = os.Readlink(path); err != nil {
				return nil, err
			}
		}

		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return nil, err
		}

		// paths in a .deb are relative to the package root
		hdr.Name = "./" + filepath.ToSlash(rel)
		if info.IsDir() {
			hdr.Name += "/"
			hdr.Mode = 0755
		} else {
			hdr.Mode = int64(mode(rel, info.Mode()))
		}

		// everything a package installs is owned by root.  Setting this here
		// is what replaces fakeroot: nothing needs to believe the staged
		// files are root owned, because the archive simply says they are.
		hdr.Uid = 0
		hdr.Gid = 0
		hdr.Uname = "root"
		hdr.Gname = "root"

		// drop anything host specific that would vary between builds
		hdr.ModTime = modTime
		hdr.AccessTime = time.Time{}
		hdr.ChangeTime = time.Time{}
		hdr.Format = tar.FormatGNU

		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("couldn't write the tar header for %s: %s", rel, err)
		}

		if !info.Mode().IsRegular() {
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}

		n, err := io.Copy(tw, f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("couldn't write %s into the archive: %s", rel, err)
		}

		// a file that changed size under us would otherwise desync the archive
		if n != info.Size() {
			return nil, fmt.Errorf("%s changed size while being packaged (read %d bytes, expected %d)", rel, n, info.Size())
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
