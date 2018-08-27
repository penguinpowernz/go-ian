# go-ian

Simple debian package building and management named in memory of the late Ian Murdock, founder of the Deb**ian** project.

The purpose of this tool is to decrease the overhead in maintaining a debian package stored 
in a git repository. It tries to mimic the CLI of other popular tools such as git and bundler.
It is intended to be helpful when integrating other build tools/systems and with CI/CD.

It has been ported to golang from the [ruby project of the same name](https://github.com/penguinpowernz/ian).

You can download binaries and Debian packages from the [releases](https://github.com/penguinpowernz/go-ian/releases) page.

## Requirements

I shell out a bit to save time, will eventually make things more native.  For now, need the following tools:

* dpkg-deb
* fakeroot
* rsync

This should do it.

    sudo apt-get install fakeroot dpkg-dev rsync coreutils findutils

## Installation

Simple to build / install, provided you have go setup:

    go get github.com/penguinpowernz/go-ian
    go install github.com/penguinpowernz/go-ian/cmd/ian
        
I will provide some binaries once I figure out how github releases section works.
    
## Usage

This tool is used for working with what Debian called "Binary packages" - that is ones that have the `DEBIAN`
folder in capitals to slap Debian packages together quickly. It uses dpkg-deb -b in the background which most
Debian package maintainers frown at but it is suitable enough for rolling your own packages quickly, and it
scratches an itch.

### Creation/Initializing

    ian new my-package
 
Analogous to `bundle new gemname` or `rails new appname` this will create a folder called `my-package` in the
current folder and initialize it with the appropriate debian files.

    ian init
    
Analagous to `git init` this will do the same as `new` but do it in the current folder.

Now you will see you have a `DEBIAN` folder with a `control` and `postinst` file.

### Info

    ian info
    
This will simply dump the control file contents out.
 
### Set fields in the control file

The architecture and the version can be set quickly in this manner.  Other fields are not (yet) supported.

    ian set -a amd64
    ian set -v 1.2.3-test

You can also use increments on semantic versions like so:

    ian set -v +M    # increment the Major number
    ian set -v +m    # increment the minor number
    ian set -v +p    # increment patch level

### Packaging

    ian pkg [-b]
    
The one you came here for.  Packages the repo in a debian package, excluding junk files like `.git` and `.gitignore`, 
moves root files (like `README.md`) to a `/usr/share/doc` folder so you don't dirty your root partition on install.  
The package will be output to a `pkg` directory in the root of the repo.  It will also generate the md5sums file
and calculate the package size proir to packaging.  By adding a `-b` flag it will run the build script before
packaging.

### Build

    ian build

This will run the build script found in `DEBIAN/build` parsing it the following arguments:

- root directory of the package git repository
- architecture from the control file
- version from the control file

It can do whatever things you need it to do to prepare for the packaging such as building binaries, etc.

### Push

    ian push [name]

Setup scripts to run in a file called `.ianpush` in the repo root and running `ian push` will run all the lines in
the file as commands with the current package.  The package filename will be appended to each command unless `$PKG`
is found on the line, in which case that will be replaced with the package filename.  Alternatively the package
filename can be given as an argument.

### Other

Some other commands:

    ian install     # installs the current package
    ian excludes    # shows the excluded files
    ian size        # calculates the package size (in kB)
    ian -v          # prints the ian version
    ian version     # prints the package version
    ian versions    # prints all known versions
    ian deps        # prints the dependencies line by line
    bpi		        # run build, pkg, install
	pi		        # run pkg, install
	pp		        # run pkg, push
	bp		        # run build, pkg
	bpp		        # run build, pkg push

You can also use the envvar `IAN_DIR` in the same way that you would use `GIT_DIR` - that is, to do stuff
with ian but from a different folder location.

## Library Usage

[![GoDoc](https://godoc.org/github.com/penguinpowernz/go-ian/debian/control?status.svg)](https://godoc.org/github.com/penguinpowernz/go-ian/debian/control)

The Debian package `Control` struct could come in handy for others.  As a quick overview here's what it can do:

- `Parse([]byte) (Control, error)` - parse the bytes from the control file
- `Read(string) (Control, error)` - read the given file and parse it's contents
- `Default() (Control)` - a default package control file
- `ctrl.Filename() string` - the almost Debian standard filename (missing distro name)
- `ctrl.String() string` - render the control file as a string
- `ctrl.WriteFile(string) error` - write the string into the given filename

Plus the exported fields on the `Control` struct that mirror the dpkg field names.

For more information please check the godocs.

## Dogfooding

The debian package source for Ian is actually managed by Ian in the folder `dpkg`. So you can build the debian
package for ian, using ian.  Give it a try!

    go get github.com/penguinpowernz/go-ian
    go install github.com/penguinpowernz/go-ian/cmd/ian
    cd $GOPATH/src/github.com/penguinpowernz/go-ian/dpkg
    ian build
    ian pkg
    sudo $GOBIN/ian install # or sudo dpkg -i pkg/ian_v1.0.0_amd64.deb

## TODO

- [ ] tests
- [x] add help page
- [ ] add subcommands help
- [ ] releasing
- [x] pushing
- [x] test pushing
- [x] ignore file
- [ ] allow specifying where to output the package to after building
- [ ] deps management
- [x] running of a build script
- [x] install after packaging
- [ ] package a specific version
- [ ] optional semver enforcement
- [ ] utilize rules file
- [ ] support copyright file
- [ ] support changelog
- [x] don't shell out for md5sums
- [ ] don't shell out for rsync
- [x] don't shell out for find
- [x] pull maintainer from git config

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/penguinpowernz/ian.

## In Memory Of

In memory of Ian Ashley Murdock (1973 - 2015) founder of the Debian project.
