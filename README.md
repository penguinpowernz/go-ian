# go-ian

Simple debian package building and management named in memory of the late Ian Murdock, founder of the Deb**ian** project.

The purpose of this tool is to decrease the overhead in maintaining a debian package stored 
in a git repository. It tries to mimic the CLI of other popular tools such as git and bundler.
It is intended to be helpful when integrating other build tools/systems and with CI/CD.

It has been ported to golang from the [ruby project of the same name](https://github.com/penguinpowernz/ian).

You can download binaries and Debian packages from the [releases](https://github.com/penguinpowernz/go-ian/releases) page.

## Upgrading to v4.0.0

**v4.0.0 is a significant change and existing packages will not build until they are migrated.**

What goes into a package is now decided by the `DEBIAN/md5sums` manifest, rather than by sweeping the whole
repo and filtering it through `.ianignore`. This makes the file list explicit and auditable: the sums of
files that aren't themselves committed (built binaries, for instance) are recorded in the repo, so a released
`.deb` can be checked against the source it was built from.

What changed:

* only files registered with `ian add` are included in the package
* `.ianignore` is no longer used, and the `ian excludes` command is gone
* `ian files`, `ian sums` and `ian size` are gone — `ian pkg -n` prints the file list without building
* bare files in the repo root are no longer swept into `/usr/share/doc` automatically — they are registered
  explicitly with `ian doc`, which records them in `DEBIAN/docfiles`
* `ian pkg` fails if a registered file no longer matches its recorded sum (pass `-k` to downgrade to a warning)

To migrate an existing package, run this in the package directory:

    ian migrate      # shows what it would register, writes nothing
    ian migrate -f   # applies it, and deletes the now-unused .ianignore

It derives the file list using the old rules — your `.ianignore` patterns plus ian's built-in defaults — so the
result should match what the previous version packaged, with bare root files becoming doc files. Review the
output, then commit `DEBIAN/md5sums` and `DEBIAN/docfiles`. `ian status` will show you which registered files
have since drifted.

## Requirements

Building a package needs nothing but the `ian` binary itself.  The package is written in process using only
the Go standard library, so `dpkg-deb`, `fakeroot`, `md5sum` and `du` are no longer required.

Two optional features still call out to external tools:

* `ian push` runs whatever transport you configure in `.ianpush` (`scp`, `rsync`, a custom script...)
* `git` is used to default the maintainer from `git config`, and for `ian set -V` (`git describe`)

Neither is on the package build path.

## Installation

Simple to build / install, provided you have go setup:

    go get github.com/penguinpowernz/go-ian
    go install github.com/penguinpowernz/go-ian/cmd/ian

Or you can download a pre-built binary or Debian package from [the releases page](https://github.com/penguinpowernz/go-ian/releases).

## Usage

This tool is used for working with what Debian called "Binary packages" - that is ones that have the `DEBIAN`
folder in capitals to slap Debian packages together quickly. It writes the `.deb` directly rather than going
through `dpkg-buildpackage`, which most Debian package maintainers frown at, but it is suitable enough for
rolling your own packages quickly, and it scratches an itch.

### Initializing

    ian init

Analagous to `git init`, this turns the current folder into an ian package.

Now you will see you have a `DEBIAN` folder containing a `control` file, an empty `md5sums` manifest, a
`docfiles` list, and `preinst`/`postinst`/`prerm`/`postrm` maintainer scripts.

### Info

    ian info

This will simply dump the control file contents out.  There are flags for version, priority, section, architecture etc.
 
### Set fields in the control file

The architecture and the version can be set quickly in this manner.  Other fields are not (yet) supported.

    ian set -a amd64
    ian set -v 1.2.3-test

### Registering files for the package

    ian add usr/bin/myapp etc/myapp/config.yml

`ian` decides what goes into the package using the `DEBIAN/md5sums` manifest: **only files listed there are
included**, so the Debian packaging metadata can live happily alongside your source without dragging unwanted
files into the package.  `ian add` computes the MD5 sum of each given file and records it in `DEBIAN/md5sums`
(standard Debian format, paths relative to the package root).  Re-running `ian add` on a file updates its sum.

### Packaging

    ian pkg

The one you came here for.  Stages the files listed in `DEBIAN/md5sums` into a debian package, copies the
`DEBIAN/md5sums` manifest in verbatim (so `debsums` works normally) and calculates the package size prior to
packaging.  The package will be output to a `pkg` directory in the root of the repo.

Before staging, every file's MD5 sum is verified against the manifest.  If any file is missing or its sum no
longer matches, the build **fails** — protecting you from shipping a file that changed since it was registered.
Pass `-k` / `--insecure` to downgrade these failures to warnings and build anyway.

After the package is written it is read back and checked: the `md5sums` shipped inside the `.deb` is compared
against the committed manifest, and every file is extracted and rehashed, so a file that changed between being
verified and being packaged is caught. Pass `-K` / `--no-package-check` to skip this, or `-k` to downgrade
its failures to warnings.

By default the file list is printed before building. Use `-q` to suppress it, or `-n` for a dry run that prints
the files without building.

Builds are reproducible: the same staged tree always produces a byte identical `.deb`. Archive timestamps are
pinned rather than taken from the files on disk, so you can build the same commit twice — ideally on two
different machines — and compare the hashes. Set `SOURCE_DATE_EPOCH` to choose the timestamp.

### Push

    ian push [target]

Setup scripts to run in a file called `.ianpush` in the repo root and running `ian push` will run all the lines in
the file as commands with the current package.  The package filename will be appended to each command unless `$PKG`
is found on the line, in which case that will be replaced with the package filename.  Also the target name can be
given as an argument to push to specfic targets (supports globbing).

    package_cloud push user/app-testing/debian/wheezy
    stable: package_cloud push user/app-stable/debian/wheezy
    devbox: scp $PKG root@192.168.1.200:~
    s3: aws s3 cp $PKG s3://mybucket/dpkg/

Note that targets requiring input will fail as there is no terminal attached to the command.  For SCP, it is recommended
to use the SSH config files to your advantage.

### Other

Use `-h` to get help on commands and their available flags.

Some other commands:

    ian -d dpkg pkg # uses the folder called `dpkg` as the package root
    ian add <file>  # registers a file (and its md5sum) for inclusion
    ian rm <file>   # unregisters a file, leaving it on disk
    ian doc         # lists doc files and where they install to
    ian doc <file>  # registers a file to install into /usr/share/doc
    ian status      # shows which registered files have changed or gone missing
    ian migrate     # migrates a pre-v4.0.0 package to the manifest format
    ian pkg -n      # lists the files that would be included, without building
    ian -V          # prints the ian version
    ian deps        # prints the dependencies line by line

You can also use the envvar `IAN_DIR` instead of `-d` in the same way that you would use `GIT_DIR` - that is, to do stuff
with ian but from a different folder location.

Use `ian pkg -x` to show debug logs while building.

## Library Usage

[![GoDoc](https://godoc.org/github.com/penguinpowernz/go-ian/debian/control?status.svg)](https://godoc.org/github.com/penguinpowernz/go-ian/debian/control)

The Debian package `Control` struct could come in handy for others.  As a quick overview here's what it can do:

* `Parse([]byte) (Control, error)` - parse the bytes from the control file
* `Read(string) (Control, error)` - read the given file and parse it's contents
* `Default() (Control)` - a default package control file
* `ctrl.Filename() string` - the almost Debian standard filename (missing distro name)
* `ctrl.String() string` - render the control file as a string
* `ctrl.WriteFile(string) error` - write the string into the given filename

Plus the exported fields on the `Control` struct that mirror the dpkg field names.

For more information please check the godocs.

## Dogfooding

The debian package source for Ian is actually managed by Ian in the folder `dpkg`. So you can build the debian
package for ian, using ian.  Give it a try!

    go get github.com/penguinpowernz/go-ian
    go install github.com/penguinpowernz/go-ian/cmd/ian
    cd $GOPATH/src/github.com/penguinpowernz/go-ian/dpkg
    ian pkg
    sudo dpkg -i pkg/ian_*.deb

## TODO

* [ ] more tests
* [x] add help page
* [x] add subcommands help
* [x] pushing
* [x] test pushing
* [x] ignore file
* [x] allow specifying where to output the package to after building
* [x] deps management
* [ ] install after packaging
* [ ] package a specific version
* [ ] optional semver enforcement
* [ ] utilize rules file
* [ ] support copyright file
* [ ] support changelog
* [x] don't shell out for md5sums
* [x] don't shell out for rsync
* [x] don't shell out for find
* [x] don't shell out for dpkg-deb
* [x] don't shell out for fakeroot
* [x] don't shell out for du
* [x] pull maintainer from git config

## Contributor Code of Conduct

This project adheres to No Code of Conduct.  We are all adults.  We accept anyone's contributions.  Nothing else matters.

For more information please visit the [No Code of Conduct](https://github.com/domgetter/NCoC) homepage.

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/penguinpowernz/ian.

## In Memory Of

In memory of Ian Ashley Murdock (1973 - 2015) founder of the Debian project.
