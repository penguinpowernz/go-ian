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
	"github.com/penguinpowernz/go-ian/util/tell"
)

var version = "v0.8.1"

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
		name, err := ian.Package(ctrl, dir, "")
		fatalIf(err, "packaging failed")
		fmt.Println(name)

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

	case "info":
		ctrl := readCtrl(dir)
		fmt.Println(ctrl.String())

	case "push":
		ensureInit(dir)
		pushFile := filepath.Join(dir, ".ianpush")
		_, err := os.Stat(pushFile)
		if os.IsNotExist(err) {
			tell.Fatalf("No .ianpush file exists")
		}

		ctrl := readCtrl(dir)
		pkg := filepath.Join(dir, "pkg", ctrl.Filename())

		if len(os.Args) == 3 {
			pkg = os.Args[2]
		}

		tell.IfFatalf(ian.Push(pushFile, pkg), "pushing failed")

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

func printHelp() {
	fmt.Println(`Usage: ian <command> [options]
	-v,             Print the version
	-h, --help      Display this help message.

Available commands:

	new        Create a new Debian package from scratch
	init       Initialize the current folder as a Debian package
	pkg        Build a Debian package
	push       Push the latest debian package up
	set        Modify the Debian control file
	info       Print information for this package
	deps       Print dependencies for this package
	versions   Show all the known versions
	version    Print the current versions
`)

	// build      Run the script at DEBIAN/build
	// install    Build and install a Debian package
	// release    Release the current or new version
}
