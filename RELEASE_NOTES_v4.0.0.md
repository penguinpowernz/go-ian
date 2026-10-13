# go-ian v4.0.0

**This is a breaking release. Existing packages will not build until they are migrated** — see
[Migrating](#migrating) below.

Two things changed in v4: what goes into a package is now an explicit, committed manifest rather than a
sweep of the repo, and the `.deb` is written in process by Go instead of by shelling out to `dpkg-deb`
and `fakeroot`.

## Explicit manifests instead of `.ianignore`

Previously `ian pkg` swept the whole repo and filtered it through `.ianignore`. Now only paths registered
in `DEBIAN/md5sums` are staged. The file list is explicit and auditable, and because the sums of files
that aren't themselves committed (built binaries, for instance) are recorded in the repo, a released
`.deb` can be checked against the source it was built from.

* `ian add <path>` registers a file; `ian rm <path>` takes it back out
* `ian status` shows registered files that have drifted from their recorded sums
* `ian add -u` re-hashes every already-registered file at once
* `ian pkg` fails if a registered file no longer matches its sum (`-k` downgrades this to a warning)
* `.ianignore` is no longer read, and `ian excludes` is gone

Doc files are explicit too. Bare files in the repo root are no longer swept into `/usr/share/doc`;
register them with `ian doc`, which records them in `DEBIAN/docfiles`.

## Per-architecture sums

A cross-compiled binary is different bytes on every architecture, so a single `md5sums` could only ever
vouch for whichever arch was built last. Sums are now kept per architecture in `DEBIAN/md5sums.<arch>`,
so every architecture's manifest can be committed side by side and each build verifies strictly against
its own:

    ian set -a amd64 && cp build/app.amd64 usr/bin/app && ian add usr/bin/app   # -> DEBIAN/md5sums.amd64
    ian set -a i386  && cp build/app.i386  usr/bin/app && ian add usr/bin/app   # -> DEBIAN/md5sums.i386

`ian pkg` reads the manifest for the architecture in the control file and ships it inside the `.deb` as
plain `md5sums`, so `debsums` works normally and the other architectures' manifests stay out of the
package. An architecture of `all` means contents don't vary, so it uses the plain `DEBIAN/md5sums`.

Files that are identical on every architecture — a config file, a systemd unit, a script — don't need
registering once per arch: `ian add -a` records them in every manifest at once, and `ian rm -a` removes
them from every manifest in one go.

ian itself is now released this way; the project `Makefile` is the worked example.

## Go-native packaging

The archive is written in process with the Go standard library. A build needs nothing but the `ian`
binary — no `dpkg-deb`, `fakeroot`, `md5sum` or `du`, and nothing from the host's `/usr/bin` on the build
path. Archive entries are written as uid/gid 0 regardless of who built them, which is what makes
`fakeroot` unnecessary.

Two optional features still call out, neither on the build path: `ian push` runs the transport you
configure in `.ianpush`, and `git` is used to default the maintainer from `git config` and for
`ian set -V`.

## Verification and hardening

Contents are verified at three stages: sums are checked before staging, the staged tree is rechecked
before the archive is sealed, and the finished `.deb` is re-opened — its `md5sums` compared to the
committed manifest and every file extracted and rehashed. Each stage also fails on files that are
present but *not* registered, so an injected extra file is caught, not just a modified one. The readback
uses ian's own reader rather than `dpkg-deb`, since asking the tool that wrote the package whether the
package is correct proves little. `-k` downgrades these to warnings and `-K` skips the readback; both are
opt-in.

Manifest paths are validated going into and coming out of the manifest, since it's a hand-editable file
in git: absolute paths, `..` traversal and anything under `DEBIAN/` are rejected, and staging re-confines
every source and destination path to the repo and staging dir respectively. `.ianpush` commands are
executed via `exec` with an argv and never through a shell, so a package filename cannot be interpreted
as shell syntax.

See the Security section of the README for the full rationale.

## Other changes

* `ian set -V` with no argument takes the version from `git describe`
* `ian pkg -n` prints the file list without building, replacing the removed `ian files`, `ian sums` and
  `ian size`
* `ian status` output is colorized, and warnings and errors print in red
* `ian pkg -x` debug output reformatted
* unused commands removed; dependencies updated
* release artifacts now ship `sha256sums`

## Migrating

Run this in the package directory:

    ian migrate      # shows what it would register, writes nothing
    ian migrate -f   # applies it, and deletes the now-unused .ianignore

`ian migrate` derives the file list using the old rules — your `.ianignore` patterns plus ian's built-in
defaults — so the result should match what the previous version packaged, with bare root files becoming
doc files. Review the output, then commit `DEBIAN/md5sums` and `DEBIAN/docfiles`. `ian status` will show
you which registered files have since drifted.

## Removed

| Removed | Use instead |
| --- | --- |
| `.ianignore` | `ian add` / `DEBIAN/md5sums` |
| `ian excludes` | — |
| `ian files` | `ian pkg -n` |
| `ian sums` | `ian pkg -n` |
| `ian size` | `ian pkg -n` |
| automatic `/usr/share/doc` sweep | `ian doc` / `DEBIAN/docfiles` |
