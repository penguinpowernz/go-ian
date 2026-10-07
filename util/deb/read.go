package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractControl writes the control files from the .deb at path into dir.
//
// This is the in process equivalent of `dpkg-deb --control`.  Reading the
// package back with our own reader rather than dpkg-deb is what makes the
// post build check meaningful: a verification that asks the same tool that
// wrote the package whether the package is correct establishes very little.
func ExtractControl(path, dir string) error {
	return extractMember(path, "control.tar", dir)
}

// ExtractData writes the filesystem tree from the .deb at path into dir, the
// in process equivalent of `dpkg-deb --extract`.
func ExtractData(path, dir string) error {
	return extractMember(path, "data.tar", dir)
}

// extractMember finds the member whose name starts with prefix and untars it
// into dir
func extractMember(path, prefix, dir string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	members, err := readAr(f)
	if err != nil {
		return fmt.Errorf("couldn't read %s: %s", path, err)
	}

	for _, m := range members {
		if !strings.HasPrefix(m.Name, prefix) {
			continue
		}

		r, err := decompress(m)
		if err != nil {
			return fmt.Errorf("couldn't decompress %s from %s: %s", m.Name, path, err)
		}

		return untar(r, dir)
	}

	return fmt.Errorf("no %s member in %s", prefix, path)
}

// decompress wraps the member body in a reader for its compression format,
// which the member's extension names
func decompress(m arMember) (io.Reader, error) {
	switch {
	case strings.HasSuffix(m.Name, ".gz"):
		return gzip.NewReader(bytes.NewReader(m.Body))
	case strings.HasSuffix(m.Name, ".tar"):
		return bytes.NewReader(m.Body), nil
	default:
		// xz, zstd and bzip2 are all legal in a .deb but need a dependency
		// to read, and ian only ever writes gzip
		return nil, fmt.Errorf("unsupported compression for member %q", m.Name)
	}
}

// untar extracts the tar stream into dir.
//
// Only directories, regular files and symlinks are extracted; device nodes and
// fifos are skipped, as a .deb ian built never contains them and creating them
// would need privileges.  Entry paths are confined to dir, so a package with a
// traversing path cannot write outside the extraction directory.
func untar(r io.Reader, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("couldn't read the archive: %s", err)
		}

		// a .deb names its entries ./usr/bin/foo
		name := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "./")
		if name == "" || name == "/" {
			continue
		}

		target, err := confine(root, name)
		if err != nil {
			return fmt.Errorf("refusing to extract %s: %s", hdr.Name, err)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode).Perm())
			if err != nil {
				return err
			}

			// the header's size bounds the copy, so a stream claiming one size
			// and carrying more cannot fill the disk
			n, err := io.Copy(f, io.LimitReader(tr, hdr.Size))
			f.Close()
			if err != nil {
				return fmt.Errorf("couldn't extract %s: %s", hdr.Name, err)
			}
			if n != hdr.Size {
				return fmt.Errorf("%s is truncated in the archive (got %d bytes, header says %d)", hdr.Name, n, hdr.Size)
			}

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			// the link is not resolved here, so a link pointing outside the
			// extraction dir is created but never followed by the verifier,
			// which only ever reads regular files it found by walking
			os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
}

// confine joins rel onto root, returning an error if the result would escape
// root through .. or an absolute path
func confine(root, rel string) (string, error) {
	target := filepath.Join(root, rel)

	if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside %s", rel, root)
	}

	return target, nil
}
