// Package deb reads and writes binary Debian packages without shelling out to
// dpkg-deb or fakeroot.
//
// A binary .deb is an ar archive holding exactly three members, in order:
//
//	debian-binary    the format version, "2.0\n"
//	control.tar.gz   the DEBIAN control directory
//	data.tar.gz      the filesystem tree the package installs
//
// Writing these in process means a build depends only on the Go toolchain and
// its standard library, so a tampered dpkg-deb or fakeroot on the build host
// cannot influence the package.  It also puts every tar header field under our
// control, which is what makes a reproducible build possible: dpkg-deb picks
// mtimes and orderings of its own that we otherwise could not pin.
//
// Writing the ownership metadata ourselves is also what removes the need for
// fakeroot.  fakeroot exists only to convince dpkg-deb that files staged by an
// unprivileged user are owned by root; when we write the tar headers we simply
// set uid and gid to 0.
package deb

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// arMagic is the global header every ar archive starts with
const arMagic = "!<arch>\n"

// arFileMagic terminates each member header
const arFileMagic = "`\n"

// arHeaderSize is the size of a member header in bytes
const arHeaderSize = 60

// arMember is one file within an ar archive
type arMember struct {
	Name string
	Body []byte
}

// writeAr writes the members as an ar archive in the order given.
//
// Only the common format is emitted: a 16 byte name, a decimal mtime, uid, gid
// and octal mode, a decimal size and the member magic, each field space padded
// to a fixed width.  Member bodies are padded to an even offset.  dpkg requires
// the three .deb members to have short ASCII names, so the long name extensions
// (BSD's #1/ and GNU's string table) are deliberately not implemented.
func writeAr(w io.Writer, members []arMember) error {
	if _, err := io.WriteString(w, arMagic); err != nil {
		return err
	}

	for _, m := range members {
		if len(m.Name) > 16 {
			return fmt.Errorf("ar member name %q is longer than 16 bytes", m.Name)
		}

		// mtime, uid, gid and mode are fixed rather than taken from the host:
		// nothing in a .deb reads them, and pinning them keeps the archive
		// byte for byte reproducible
		hdr := fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10d%s",
			m.Name, 0, 0, 0, 0644, len(m.Body), arFileMagic)

		if len(hdr) != arHeaderSize {
			return fmt.Errorf("built a malformed ar header for %q (%d bytes, want %d)", m.Name, len(hdr), arHeaderSize)
		}

		if _, err := io.WriteString(w, hdr); err != nil {
			return err
		}

		if _, err := w.Write(m.Body); err != nil {
			return err
		}

		// members start on an even offset, so an odd sized body is padded
		if len(m.Body)%2 == 1 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
	}

	return nil
}

// readAr reads an ar archive, returning its members in the order they appear.
//
// Member bodies are read into memory.  The archives this handles are .debs
// whose members are a four byte version string and two compressed tarballs, so
// this is bounded by the size of the package being verified.
func readAr(r io.Reader) ([]arMember, error) {
	magic := make([]byte, len(arMagic))
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("couldn't read the ar magic: %s", err)
	}
	if string(magic) != arMagic {
		return nil, fmt.Errorf("not an ar archive (bad magic %q)", magic)
	}

	var members []arMember

	for {
		hdr := make([]byte, arHeaderSize)
		_, err := io.ReadFull(r, hdr)
		if err == io.EOF {
			return members, nil
		}
		// a trailing newline from a padded final member reads as a short header
		if err == io.ErrUnexpectedEOF {
			return members, nil
		}
		if err != nil {
			return nil, fmt.Errorf("couldn't read an ar member header: %s", err)
		}

		if string(hdr[58:60]) != arFileMagic {
			return nil, fmt.Errorf("malformed ar member header (bad magic %q)", hdr[58:60])
		}

		// ar pads names with spaces, and GNU ar terminates them with a slash
		name := strings.TrimRight(strings.TrimSpace(string(hdr[0:16])), "/")

		size, err := strconv.ParseInt(strings.TrimSpace(string(hdr[48:58])), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("malformed size for ar member %q: %s", name, err)
		}
		if size < 0 {
			return nil, fmt.Errorf("negative size for ar member %q", name)
		}

		body := make([]byte, size)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, fmt.Errorf("couldn't read the body of ar member %q: %s", name, err)
		}

		members = append(members, arMember{Name: name, Body: body})

		// skip the pad byte after an odd sized member
		if size%2 == 1 {
			if _, err := io.ReadFull(r, make([]byte, 1)); err != nil && err != io.EOF {
				return nil, fmt.Errorf("couldn't read the padding after ar member %q: %s", name, err)
			}
		}
	}
}
