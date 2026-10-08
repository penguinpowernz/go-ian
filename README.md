# go-ian

Simple, auditable and secure debian package building and management named in memory of the 
late Ian Murdock, founder of the Deb**ian** project.

The purpose of this tool is to decrease the overhead in maintaining a debian package stored 
in a git repository. It tries to mimic the CLI of other popular tools such as git and bundler.
It is intended to be helpful when integrating other build tools/systems and with CI/CD.

It has been ported to golang from my [ruby project of the same name](https://github.com/penguinpowernz/ian).

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
* sums are kept per architecture in `DEBIAN/md5sums.<arch>`, so a package released for several architectures can
  commit all of them — see [Multiple architectures](#multiple-architectures)

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

Control file fields can be set from the command line:

    ian set -a amd64
    ian set -v 1.2.3-test
    ian set -V  # this will set the version from the git tags

| Flag | Long | Sets |
| --- | --- | --- |
| `-n` | `--name` | the package name |
| `-a` | `--arch` | the architecture |
| `-v` | `--version` | the version |
| `-V` | `--git-version` | the version from `git describe` (tag-commit-dirty), with any leading `v` stripped |
| `-m` | `--maintainer` | the maintainer |
| `-D` | `--description` | the short description |
| `-L` | `--long-description` | the long description |

Several can be given at once, and each field that is set is echoed back:

    $ ian set -a amd64 -v 1.2.3
    Architecture set to amd64
    Version set to 1.2.3

If `-v` and `-V` are given together the explicit `-v` wins.  Giving no flags at all is an error.  The same
flags are accepted by `ian init`, so a package can be described as it is created.

### Git like file addition

    ian add usr/bin/myapp etc/myapp/config.yml

`ian` decides what goes into the package using the `DEBIAN/md5sums` manifest: **only files listed there are
included**, so the Debian packaging metadata can live happily alongside your source without dragging unwanted
files into the package.  `ian add` computes the MD5 sum of each given file and records it in `DEBIAN/md5sums`
(standard Debian format, paths relative to the package root).

Re-running `ian add` on a file updates its sum, so if you update something in the package it will enter your
git commit record so the package contents are auditable.

You can also remove files. Then they will no longer be included in the package when you rebuild.

    ian rm usr/bin/myapp

You can always run `ian status` to see what files have changed.

    $ ian status
    Registered and unchanged:

      ok        usr/bin/ian

    1 file(s) registered, all match the manifest.
      (use "ian add <file>..." to register more, "ian pkg" to build)

If you want to update all of the files that you have modified (that already exist in the manifest) at once just do:

    ian add -u

### Multiple architectures

I want to make it so that if you need to release the same code for multiple architectures from the same repo
you get the same benefits. A cross-compiled binary is different bytes on every architecture, so one `md5sums`
could only ever vouch for whichever arch was built last.

The sums are therefore kept per architecture in `DEBIAN/md5sums.<arch>`, which lets every architecture's sums
be committed side by side and each build verify strictly against its own:

    ian set -a amd64 && cp build/app.amd64 usr/bin/app && ian add usr/bin/app   # -> DEBIAN/md5sums.amd64
    ian set -a i386 && cp build/app.i386 usr/bin/app && ian add usr/bin/app     # -> DEBIAN/md5sums.i386

Commit all of them.  `ian pkg` reads the manifest for the architecture in the control file and ships it inside
the `.deb` as plain `md5sums`, so `debsums` works normally and the other architectures' manifests are left out
of the package.

**You can check the Makefile in this project to see how `ian` itself is released for multiple architectures from
the single repo.**

An architecture of `all` means the contents don't vary, so it uses the plain `DEBIAN/md5sums` and never gets a
qualified manifest.

Files that are identical on every architecture — a config file, a systemd unit, a script — don't need
registering once per arch.  `ian add -a` records them in every manifest at once:

    $ ian add -a etc/app.conf
    added etc/app.conf to md5sums, md5sums.amd64, md5sums.arm64

Sometimes however the only thing to do is have a separate folder in the repository for each different architecture.
This is what I do on a few projects. It adds multiple package roots but sometimes it is the cleanest way to do it
hiding the implementation behind Makefile tasks.

### Packaging

    ian pkg

The one you came here for.  Stages the files listed in the manifest for the package's architecture into a
debian package, copies that manifest in verbatim as `md5sums` (so `debsums` works normally) and calculates the
package size prior to packaging.  The package will be output to a `pkg` directory in the root of the repo.

Before staging, every file's MD5 sum is verified against the manifest.  If any file is missing or its sum no
longer matches, the build **fails** — protecting you from shipping a file that changed since it was registered.
Pass `-k` / `--insecure` to downgrade these failures to warnings and build anyway. The files are also verified
in staging right before the package is built to prevent something modifying your staging dir unnoticed.

After the package is written it is read back and checked: the `md5sums` shipped inside the `.deb` is compared
against the committed manifest, and every file is extracted and rehashed, so a file that changed between being
verified and being packaged is caught. Pass `-K` / `--no-package-check` to skip this, or `-k` to downgrade
its failures to warnings.

Builds are reproducible: the same staged tree always produces a byte identical `.deb`. Archive timestamps are
pinned rather than taken from the files on disk, so you can build the same commit twice — ideally on two
different machines — and compare the hashes. Set `SOURCE_DATE_EPOCH` to choose the timestamp.

### Push

    ian push [target]

Setup scripts to run in a file called `.ianpush` in the repo root, and running `ian push` will run all the lines in
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
    ian add -u      # re-registers every registered file that has changed
    ian rm <file>   # unregisters a file, leaving it on disk
    ian doc         # lists doc files and where they install to
    ian doc <file>  # registers a file to install into /usr/share/doc/<package-name>
    ian status      # shows which registered files have changed, and which manifest it read
    ian migrate     # migrates a pre-v4.0.0 package to the manifest format
    ian pkg -n      # lists the files that would be included, without building
    ian -V          # prints the ian version
    ian deps        # prints the dependencies, and hints how to modify

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
    cd $GOPATH/src/github.com/penguinpowernz/go-ian
    make build         # builds for amd64
    cp ian usr/bin/ian # copy it into the package location
    ./ian set -a amd64 # switch to the amd64 arch
    ./ian status
    ./ian add -u       # update it if your binary build was different
    ./ian pkg -x       # use the debug flag to see exactly how the package process works
    sudo dpkg -i pkg/ian_*.deb

Notice that README.md appears at `/usr/share/doc/ian/README.md` because it is added to the `DEBIAN/docfiles`.

## Security

Packaging is a supply chain step: a `.deb` is a tarball that installs as root. The design aims to make
what ships be exactly what was committed, and to keep the tool itself a small target.

**What goes in the package is an explicit list, not a sweep.** Only paths registered in
`DEBIAN/md5sums` are staged, so a stray key, `.env` or editor backup in the repo cannot be swept into a
release. Registered paths are validated on the way in *and* on the way out of the manifest (it is a
hand-editable file in git): absolute paths, `..` traversal and anything under `DEBIAN/` are rejected, and
staging re-confines every source and destination path to the repo and the staging dir respectively.

**Contents are verified at three stages.** Sums are checked before staging, the staged tree is rechecked
just before the archive is sealed, and the finished `.deb` is then re-opened — its `md5sums` compared to
the committed manifest and every file extracted and rehashed. Each stage also fails on files present but
*not* registered, so an injected extra file is caught, not just a modified one. The readback uses ian's
own reader rather than `dpkg-deb`, since asking the tool that wrote the package whether the package is
correct proves little. `-k` downgrades these to warnings and `-K` skips the readback — both are opt-in.

**No shell, and few external tools.** The archive is written in process with the Go standard library, so
a build needs no `dpkg-deb`, `fakeroot`, `md5sum` or `du`, and nothing from the host's `/usr/bin` is on
the build path. `ian push` is the one place that runs user-supplied commands; those come from your own
`.ianpush` and are executed via `exec` with an argv, never through a shell, so a package filename cannot
be interpreted as shell syntax.

**Permissions and ownership are normalised, not inherited.** Archive entries are written as `uid`/`gid` 0
regardless of who built them (which is what makes `fakeroot` unnecessary), only the five maintainer
scripts are written executable while other control files are forced to `0644`, and setuid, setgid and
sticky bits are dropped from installed files.

**Builds are reproducible.** Timestamps are pinned (`SOURCE_DATE_EPOCH` is honoured), entries sorted and
host-specific metadata dropped, so the same commit builds byte-identical on two machines — which lets a
third party confirm a published `.deb` came from the published source.

**Extraction is defensive.** Reading a `.deb` back confines every entry to the extraction directory,
bounds each file by its header size, and skips device nodes and fifos.

Note the manifest uses MD5 because that is what the `md5sums` control file format and `debsums` require.
MD5 is collision-prone, so treat it as drift detection against accident, not as a defence against a
motivated attacker who can write to your repo — for that, rely on git signing and review of the
committed sums.

Found a security issue? Please open an issue, or mail the maintainer for anything you'd rather not
disclose publicly.

## TODO

* [x] more tests
* [x] add help page
* [x] add subcommands help
* [x] pushing
* [x] test pushing
* [x] ignore file
* [x] allow specifying where to output the package to after building
* [x] deps management
* [ ] package a specific version using git tags
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
* [ ] honour `TMPDIR` instead of hardcoding `/tmp` for the staging and verify dirs
* [ ] stage files with a sanitised mode so a setuid bit in the repo can't reach the staging dir
* [ ] sign packages, or emit a detached signature / `SHA256SUMS` for releases

## Contributor Code of Conduct

This project adheres to No Code of Conduct.  We are all adults.  We accept anyone's contributions.  Nothing else matters.

For more information please visit the [No Code of Conduct](https://github.com/domgetter/NCoC) homepage.

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/penguinpowernz/ian.

## In Memory Of

In memory of Ian Ashley Murdock (1973 - 2015) founder of the Debian project.
