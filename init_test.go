package ian

import (
	"io/ioutil"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func fexists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func TestInit(t *testing.T) {
	Convey("given a directory", t, func() {
		dir, err := ioutil.TempDir("/tmp", "go-ian-test")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		Convey("when it is initialized", func() {
			So(fexists(dir), ShouldBeTrue)
			err = Initialize(dir)
			So(err, ShouldBeNil)

			Convey("it should have the required files in it", func() {
				So(fexists(dir+"/DEBIAN/postinst"), ShouldBeTrue)
				So(fexists(dir+"/.ianignore"), ShouldBeTrue)
				So(fexists(dir+"/DEBIAN/control"), ShouldBeTrue)
			})
		})
	})
}
