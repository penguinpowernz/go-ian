package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	ian "github.com/penguinpowernz/go-ian"
	"github.com/penguinpowernz/go-ian/debian/control"
	"github.com/penguinpowernz/go-ian/util/file"
	"github.com/penguinpowernz/go-ian/util/tell"
)

var version = "v1.0.0"

func main() {

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	if envdir := os.Getenv("IAN_DIR"); envdir != "" {
		dir = envdir
	}

	if len(os.Args) == 1 {
		printHelp()
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	case "help", "-h", "--help":
		printHelp()
		os.Exit(0)
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
		ctrl := readCtrl(dir)
		if os.Args[2] == "-b" {
			doBuild(dir, ctrl)
		}
		doPackage(dir, ctrl)

	case "set":
		ctrl := readCtrl(dir)

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

		fatalIf(ctrl.WriteFile(ian.ControlFile(dir)), "couldn't set fields")

	case "deps":
		ctrl := readCtrl(dir)
		for _, dep := range ctrl.Depends {
			fmt.Println(dep)
		}

	case "build":
		ctrl := readCtrl(dir)
		doBuild(dir, ctrl)

	case "install":
		ctrl := readCtrl(dir)
		doInstall(dir, ctrl)

	case "info":
		ctrl := readCtrl(dir)
		fmt.Println(ctrl.String())

	case "push":
		ctrl := readCtrl(dir)
		doPush(dir, ctrl)

	case "-v":
		fmt.Println("Version", version)
		fmt.Println("In memory of Ian Ashley Murdock (1973 - 2015), founder of the Debian Project")

	case "versions":
		cmd := exec.Command("/usr/bin/git", "tag")
		cmd.Stdout = os.Stdout
		tell.IfFatalf(cmd.Run(), "")

	case "version":
		ctrl := readCtrl(dir)
		fmt.Printf("%s: %s\n", ctrl.Name, ctrl.Version)

	case "bpi":
		ctrl := readCtrl(dir)
		doBuild(dir, ctrl)
		doPackage(dir, ctrl)
		doInstall(dir, ctrl)

	case "pi":
		ctrl := readCtrl(dir)
		doPackage(dir, ctrl)
		doInstall(dir, ctrl)

	case "pp":
		ctrl := readCtrl(dir)
		doPackage(dir, ctrl)
		doPush(dir, ctrl)

	case "bp":
		ctrl := readCtrl(dir)
		doBuild(dir, ctrl)
		doPackage(dir, ctrl)
		doPush(dir, ctrl)

	case "bpp":
		ctrl := readCtrl(dir)
		doBuild(dir, ctrl)
		doPackage(dir, ctrl)
		doPush(dir, ctrl)

	default:
		fmt.Println("unknown argument:", cmd)
		printHelp()
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

func readCtrl(dir string) control.Control {
	ensureInit(dir)
	ctrl, err := control.Read(ian.ControlFile(dir))
	fatalIf(err, "couldn't read control file")
	return ctrl
}

func doBuild(dir string, ctrl control.Control) {
	script := ian.ControlDir(dir, "build")
	if !file.Exists(script) {
		tell.Fatalf("script not found at %s", script)
	}
	cmd := exec.Command(script, dir, ctrl.Version, ctrl.Arch)
	cmd.Stdout = os.Stdout
	tell.IfFatalf(cmd.Run(), "")
}

func doPackage(dir string, ctrl control.Control) {
	name, err := ian.Package(ctrl, dir, "")
	fatalIf(err, "packaging failed")
	fmt.Println(name)
}

func doInstall(dir string, ctrl control.Control) {
	pkg := filepath.Join(dir, "pkg", ctrl.Filename())
	cmd := exec.Command("/usr/bin/dpkg", "-i", pkg)
	cmd.Stdout = os.Stdout
	tell.IfFatalf(cmd.Run(), "installing %s failed", pkg)
}

func doPush(dir string, ctrl control.Control) {
	pushFile := filepath.Join(dir, ".ianpush")
	_, err := os.Stat(pushFile)
	if os.IsNotExist(err) {
		tell.Fatalf("No .ianpush file exists")
	}

	pkg := filepath.Join(dir, "pkg", ctrl.Filename())

	if len(os.Args) == 3 {
		pkg = os.Args[2]
	}

	tell.IfFatalf(ian.Push(pushFile, pkg), "pushing failed")
}

func printHelp() {
	fmt.Println(`Usage: ian <command> [options]
	-v,             Print the version
	-h, --help      Display this help message.

Available commands:

	new        Create a new Debian package from scratch
	init       Initialize the current folder as a Debian package
	build      Run the script at DEBIAN/build
	pkg [-b]   Build a Debian package
	install    Install the current version of the package
	push       Push the latest debian package up
	set        Modify the Debian control file
	info       Print information for this package
	deps       Print dependencies for this package
	versions   Show all the known versions
	version    Print the current versions
	bpi		   run build, pkg, install
	pi		   run pkg, install
	pp		   run pkg, push
	bp		   run build, pkg
	bpp		   run build, pkg push
`)

	// release    Release the current or new version
}
