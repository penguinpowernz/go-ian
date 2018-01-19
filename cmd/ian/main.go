package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	ian "github.com/penguinpowernz/go-ian"
	"github.com/penguinpowernz/go-ian/debian/control"
	"github.com/penguinpowernz/go-ian/util/tell"
)

var version = "v0.8.0"

func main() {

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	if envdir := os.Getenv("IAN_DIR"); envdir != "" {
		dir = envdir
	}

	cmd := os.Args[1]

	switch cmd {
	case "init":
		fatalIf(ian.Init(dir), "")

	case "new":
		dir = os.Args[2]
		fatalIf(ian.Init(dir), "")

	case "excludes":
		for _, exc := range ian.PackageExclusions(dir) {
			fmt.Println(exc)
		}

	case "size":
		size, err := ian.CalculateSize(dir, ian.PackageExclusions(dir))
		fatalIf(err, "")
		fmt.Println(size, "kB")

	case "pkg":
		ensureInit(dir)
		ctrl, err := control.Read(ian.ControlFile(dir))
		fatalIf(err, "couldn't read control file")
		name, err := ian.Package(ctrl, dir, "")
		fatalIf(err, "packaging failed")
		fmt.Println(name)

	case "set":
		ensureInit(dir)
		fn := ian.ControlFile(dir)
		ctrl, err := control.Read(fn)
		fatalIf(err, "couldn't read control file")

		var newVer, newArch string
		fs := flag.NewFlagSet("set", flag.ContinueOnError)
		fs.StringVar(&newVer, "v", "", "set the version")
		fs.StringVar(&newArch, "a", "", "set the architecture")
		fs.Parse(os.Args[2:])

		if newArch != "" {
			ctrl.Arch = newArch
		}

		if newVer != "" {
			ctrl.Version = newVer
		}

		fatalIf(ctrl.WriteFile(fn), "couldn't set fields")

	case "info":
		ensureInit(dir)
		ctrl, err := control.Read(ian.ControlFile(dir))
		fatalIf(err, "couldn't read control file")
		fmt.Println(ctrl.String())

	case "push":
		ensureInit(dir)
		pushFile := filepath.Join(dir, ".ianpush")
		_, err := os.Stat(pushFile)
		if os.IsNotExist(err) {
			tell.Fatalf("No .ianpush file exists")
		}

		ctrl, err := control.Read(ian.ControlFile(dir))
		tell.IfFatalf(err, "couldn't read control file")
		pkg := filepath.Join(dir, "pkg", ctrl.Filename())

		if len(os.Args) == 3 {
			pkg = os.Args[2]
		}

		tell.IfFatalf(ian.Push(pushFile, pkg), "pushing failed")

	case "-v":
		fallthrough
	case "version":
		fmt.Println("Version", version)
		fmt.Println("In memory of Ian Ashley Murdock (1973 - 2015)")

	default:
		fmt.Println("unknown argument:", cmd)
		fmt.Println("Usage: ian <command> [options]")
	}
}

func fatalIf(err error, msg string) {
	if err != nil {
		log.Fatal(msg+":", err)
	}
}

func ensureNotInit(dir string) {
	if ian.IsInitialized(dir) {
		log.Fatal("already initialized")
	}
}

func ensureInit(dir string) {
	if !ian.IsInitialized(dir) {
		log.Fatal("not initialized")
	}
}
