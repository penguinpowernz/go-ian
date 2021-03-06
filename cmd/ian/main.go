package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/blang/semver"
	ian "github.com/penguinpowernz/go-ian"
	"github.com/penguinpowernz/go-ian/debian/control"
	"github.com/penguinpowernz/go-ian/util/file"
	"github.com/penguinpowernz/go-ian/util/str"
	"github.com/penguinpowernz/go-ian/util/tell"
)

var version = ""

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

	case "whoami":
		m, ok := ian.FindMaintainer()
		if !ok {
			fmt.Println("???")
		} else {
			fmt.Println(m)
		}

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
		if len(os.Args) >= 3 && os.Args[2] == "-b" {
			doBuild(dir)
		}
		doPackage(dir)

	case "set":
		doSet(dir, os.Args[2:])

	case "deps":
		doDeps(dir, os.Args[2:])

	case "build":
		doBuild(dir)

	case "install":
		doInstall(dir)

	case "info":
		doInfo(dir, os.Args[2:])

	case "push":
		slctr := ""
		if len(os.Args) == 3 {
			slctr = os.Args[2]
		}

		doPush(dir, slctr)

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
		slctr := ""
		if len(os.Args) == 3 {
			slctr = os.Args[2]
		}

		doPackage(dir)
		doPush(dir, slctr)

	case "bp":
		doBuild(dir)
		doPackage(dir)

	case "bpp":
		slctr := ""
		if len(os.Args) == 3 {
			slctr = os.Args[2]
		}

		doBuild(dir)
		doPackage(dir)
		doPush(dir, slctr)

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

func doPush(dir, slctr string) {
	pkg := readPkg(dir)
	if !file.Exists(pkg.PushFile()) {
		tell.Fatalf("No .ianpush file exists")
	}

	deb := pkg.DebFile()

	err := ian.Push(pkg.PushFile(), deb, slctr)
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
		v := pkg.Ctrl().Version
		var err error

		switch newVer {
		case "+M":
			pkg.Ctrl().Version, err = incrMajorVer(v)
		case "+m":
			pkg.Ctrl().Version, err = incrMinorVer(v)
		case "+p":
			pkg.Ctrl().Version, err = incrPatchVer(v)
		default:
			pkg.Ctrl().Version = newVer
		}

		tell.IfFatalf(err, "couldn't set version")
	}

	err := pkg.Ctrl().WriteFile(pkg.CtrlFile())
	tell.IfFatalf(err, "couldn't set fields")
}

func incrMajorVer(v string) (string, error) {
	sv, err := semver.Parse(v)
	if err != nil {
		return v, err
	}
	sv.Major++
	return sv.String(), nil
}

func incrMinorVer(v string) (string, error) {
	sv, err := semver.Parse(v)
	if err != nil {
		return v, err
	}
	sv.Minor++
	return sv.String(), nil
}

func incrPatchVer(v string) (string, error) {
	sv, err := semver.Parse(v)
	if err != nil {
		return v, err
	}
	sv.Patch++
	return sv.String(), nil
}

func doInfo(dir string, args []string) {
	ctrl := readCtrl(dir)

	var name, ver, mntr, arch, deps bool
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	fs.BoolVar(&name, "n", false, "show just the name")
	fs.BoolVar(&ver, "v", false, "show just the version")
	fs.BoolVar(&mntr, "m", false, "show just the maintainer")
	fs.BoolVar(&arch, "a", false, "show just the architecture")
	fs.BoolVar(&deps, "d", false, "show just the dependencies")
	fs.Parse(os.Args[2:])

	s := ""
	switch {
	case name:
		s = ctrl.Name
	case ver:
		s = ctrl.Version
	case mntr:
		s = ctrl.Maintainer
	case arch:
		s = ctrl.Arch
	case deps:
		s = strings.Join(ctrl.Depends, ",")
	default:
		s = ctrl.String()
	}

	fmt.Println(s)
}

func doDeps(dir string, args []string) {
	pkg := readPkg(dir)

	var add, remove string
	fs := flag.NewFlagSet("deps", flag.ContinueOnError)
	fs.StringVar(&add, "a", "", "add the dep")
	fs.StringVar(&remove, "d", "", "remove the dep")
	fs.Parse(os.Args[2:])

	if add == "" && remove == "" {
		for _, deps := range pkg.Ctrl().Depends {
			fmt.Println(deps)
		}

		return
	}

	if add != "" {
		newpkgs := str.CleanStrings(strings.Split(add, " "))
		pkg.Ctrl().Depends = append(pkg.Ctrl().Depends, newpkgs...)
	}

	if remove != "" {
		rmdeps := str.CleanStrings(strings.Split(" ", add))

		deps := strings.Join(pkg.Ctrl().Depends, " ")
		deps = " " + deps + " "
		for _, dep := range rmdeps {
			deps = strings.Replace(deps, " "+dep+" ", " ", -1)
		}

		pkg.Ctrl().Depends = str.CleanStrings(strings.Split(deps, " "))
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
	whoami     Prints your maintainer name as found in $HOME/.gitconfig
	versions   Show all the known versions
	version    Print the current versions
	bpi        run build, pkg, install
	pi         run pkg, install
	pp         run pkg, push
	bp         run build, pkg
	bpp        run build, pkg push`)

	// release    Release the current or new version
}
