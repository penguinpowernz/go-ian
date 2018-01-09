package checksum

import (
	"bytes"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"
)

// MD5 gets the md5sum of the given directories files and save the output to the given
// outfile path.  Uses find, xargs and md5sum and recurses the entire directory.
func MD5(dir, outfile string) error {
	find := exec.Command("/usr/bin/find", dir, "-type", "f")
	xargs := exec.Command("/usr/bin/xargs", "md5sum")

	r, w := io.Pipe()
	find.Stdout = w
	xargs.Stdin = r

	var buf bytes.Buffer
	xargs.Stdout = &buf

	if err := find.Start(); err != nil {
		return err
	}

	if err := xargs.Start(); err != nil {
		return err
	}

	if err := find.Wait(); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	if err := xargs.Wait(); err != nil {
		return err
	}

	data := []byte(strings.Replace(string(buf.Bytes()), dir+"/", "", -1))
	return ioutil.WriteFile(outfile, data, 0755)
}
