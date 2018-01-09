# go-ian

Simple debian package building and management named in memory of the late Ian Murdock.

The purpose of this tool is to decrease the overhead in maintaining a debian package stored 
in a git repository. It tries to mimic the CLI of other popular tools such as git and bundler.
It is intended to be helpful when integrating other build tools/systems and with CI/CD.

It has been ported to golang from the [ruby project of the same name](https://github.com/penguinpowernz/ian).

## Requirements

I shell out a bit to save time, will eventually make things more native.  For now, need the following tools:

* dpkg-deb
* fakeroot
* md5sum
* find
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

### Packaging

    ian pkg
    
The one you came here for.  Builds a package, excluding junk files like `.git` and `.gitignore`, moves root
files (like `README.md`) to a `/usr/share/doc` folder so you don't dirty your root partition on install.  The
package will be output to a `pkg` directory in the root of the repo.  It will also generate the md5sums file
and calculate the package size proir to building.

### Other

Some other commands:

    ian excludes    # shows the excluded files
    ian size        # calculates the package size (in kB)
    ian -v          # prints the version
    ian version     # prints the version

You can also use the envvar `IAN_DIR` in the same way that you would use `GIT_DIR` - that is, to do stuff
with ian but from a different folder location.

## TODO

- [ ] tests
- [ ] help
- [ ] releasing
- [ ] pushing
- [ ] ignore file
- [ ] add `pkg` to the gitignore file
- [ ] allow specifying where to output the package to after building
- [ ] deps management
- [ ] running of a build script
- [ ] install after build
- [ ] build a specific version
- [ ] more tests
- [ ] optional semver enforcement

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/penguinpowernz/ian.

## In Memory Of

In memory of Ian Ashley Murdock (1973 - 2015) founder of the Debian project.
