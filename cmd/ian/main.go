package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

	ian "github.com/penguinpowernz/go-ian"
	"github.com/penguinpowernz/go-ian/debian/control"
	"github.com/penguinpowernz/go-ian/util/file"
	"github.com/penguinpowernz/go-ian/util/str"
	"github.com/penguinpowernz/go-ian/util/tell"
)

var version = "v1.1.0"

func main() {

	if os.Getenv("DEBUG") != "" {
		ian.Debug = true
	}

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
		tell.IfFatalf(ian.Initialize(dir), "")

	case "new":
		dir = os.Args[2]
		tell.IfFatalf(ian.Initialize(dir), "")

	case "excludes":
		pkg := readPkg(dir)
		for _, exc := range pkg.Excludes() {
			fmt.Println(exc)
		}

	case "size":
		pkg := readPkg(dir)
		sizeKB, err := pkg.Size()
		tell.IfFatalf(err, "")
		fmt.Println(sizeKB, "kB")

	case "pkg":
		if os.Args[2] == "-b" {
			doBuild(dir)
		}
		doPackage(dir)

	case "set":
		doSet(dir, os.Args[2:])

	case "deps":
		ctrl := readCtrl(dir)
		for _, dep := range ctrl.Depends {
			fmt.Println(dep)
		}

	case "build":
		doBuild(dir)

	case "install":
		doInstall(dir)

	case "info":
		ctrl := readCtrl(dir)
		fmt.Println(ctrl.String())

	case "push":
		doPush(dir)

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
		doBuild(dir)
		doPackage(dir)
		doInstall(dir)

	case "pi":
		doPackage(dir)
		doInstall(dir)

	case "pp":
		doPackage(dir)
		doPush(dir)

	case "bp":
		doBuild(dir)
		doPackage(dir)

	case "bpp":
		doBuild(dir)
		doPackage(dir)
		doPush(dir)

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
	pkg, err := ian.NewPackage(dir)
	fatalIf(err, "couldn't read control file")
	return *pkg.Ctrl()
}

func readPkg(dir string) *ian.Pkg {
	ensureInit(dir)
	pkg, err := ian.NewPackage(dir)
	tell.IfFatalf(err, "couldn't read control file")
	return pkg
}

func doBuild(dir string) {
	p := readPkg(dir)
	if !file.Exists(p.BuildFile()) {
		tell.Fatalf("build script not found at %s", p.BuildFile())
	}

	cmd := p.BuildCommand()
	tell.Debugf("running: %s", str.CommandString(cmd))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	tell.IfFatalf(cmd.Run(), "")
}

func doPackage(dir string) {
	pkg := readPkg(dir)
	pkgr := ian.DefaultPackager()
	outfile, err := pkgr.Build(pkg)
	tell.IfFatalf(err, "packaging failed")
	fmt.Println(outfile)
}

func doInstall(dir string) {
	pkg := readPkg(dir)
	cmd := exec.Command("/usr/bin/dpkg", "-i", pkg.DebFile())
	cmd.Stdout = os.Stdout
	tell.IfFatalf(cmd.Run(), "installing %s failed", pkg.DebFile())
}

func doPush(dir string) {
	pkg := readPkg(dir)
	if !file.Exists(pkg.PushFile()) {
		tell.Fatalf("No .ianpush file exists")
	}

	deb := pkg.DebFile()
	if len(os.Args) == 3 {
		deb = os.Args[2]
	}

	err := ian.Push(pkg.PushFile(), deb)
	tell.IfFatalf(err, "pushing failed")
}

func doSet(dir string, args []string) {
	pkg := readPkg(dir)
	var newVer, newArch string
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.StringVar(&newVer, "v", "", "set the version")
	fs.StringVar(&newArch, "a", "", "set the architecture")
	fs.Parse(os.Args[2:])

	if newArch != "" {
		pkg.Ctrl().Arch = newArch
	}

	if newVer != "" {
		pkg.Ctrl().Version = newVer
	}

	err := pkg.Ctrl().WriteFile(pkg.CtrlFile())
	tell.IfFatalf(err, "couldn't set fields")
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
	bpi        run build, pkg, install
	pi         run pkg, install
	pp         run pkg, push
	bp         run build, pkg
	bpp        run build, pkg push
`)

	// release    Release the current or new version
}
