# Release builds take their version from DEBIAN/control rather than git, and
# are built without any VCS stamping, so the binary depends only on the source
# and that version string.  Committing the md5sums a release build produces
# therefore doesn't change the binary, and the commit holding those sums can be
# tagged and rebuilt to the same bytes (see make verify).
VERSION=$(shell sed -n 's/^Version: //p' DEBIAN/control)
LDFLAGS=-trimpath -buildvcs=false -ldflags "-X main.Version=${VERSION}"
export CGO_ENABLED=0

# Dev builds are free to stamp the git describe output, since nobody expects
# their sums to round-trip.
DEV_VERSION=$(shell git describe --tags --always --dirty|tr -d 'v')
DEV_LDFLAGS=-ldflags "-X main.Version=${DEV_VERSION}"

# the architectures a release is built for, and the suffix of the binary that
# build_all produces for each one
ARCHES=i386 armhf amd64 arm64 armel

default: build

# a dev package for amd64; this rewrites the control Version with git describe,
# so set a real version (ian set -v) before running a release
amd64: local
	GOOS=linux GOARCH=amd64 go build ${DEV_LDFLAGS} -o ./release/ian.amd64.Linux ./cmd/ian
	./ian set -a amd64
	cp ./release/ian.amd64.Linux usr/bin/ian
	./ian add usr/bin/ian
	./ian set -V
	./ian pkg

# refuse to build a release whose version came from git describe, eg after a
# dev build has rewritten DEBIAN/control
check_version:
	@test -n "${VERSION}" || { echo "no Version in DEBIAN/control"; exit 1; }
	@if echo "${VERSION}" | grep -qE -- '-g[0-9a-f]+|-dirty|^[0-9a-f]+$$'; then \
		echo "DEBIAN/control Version '${VERSION}' looks like git describe output;"; \
		echo "set the release version with: ./ian set -v <version>"; \
		exit 1; \
	fi

build_all: check_version
	GOOS=linux GOARCH=arm GOARM=7 go build ${LDFLAGS} -o ./release/ian.armhf.Linux ./cmd/ian
	sha256sum ./release/ian.armhf.Linux | sed 's/release\///' > ./release/ian.armhf.Linux.sha256sum
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o ./release/ian.arm64.Linux ./cmd/ian
	sha256sum ./release/ian.arm64.Linux | sed 's/release\///' > ./release/ian.arm64.Linux.sha256sum
	GOOS=linux GOARCH=arm GOARM=6 go build ${LDFLAGS} -o ./release/ian.armel.Linux ./cmd/ian
	sha256sum ./release/ian.armel.Linux | sed 's/release\///' > ./release/ian.armel.Linux.sha256sum
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ./release/ian.amd64.Linux ./cmd/ian
	sha256sum ./release/ian.amd64.Linux | sed 's/release\///' > ./release/ian.amd64.Linux.sha256sum
	GOOS=linux GOARCH=386 go build ${LDFLAGS} -o ./release/ian.i386.Linux ./cmd/ian
	sha256sum ./release/ian.i386.Linux | sed 's/release\///' > ./release/ian.i386.Linux.sha256sum

build: local
local:
	go build ${DEV_LDFLAGS} ./cmd/ian

release: local pkg_all

clean:
	rm -fr pkg/*
	rm -fr release

# Build every architecture, registering each binary against that arch's own
# manifest (DEBIAN/md5sums.<arch>) so every build verifies strictly against the
# sums committed for it.  The md5sums.* files this leaves behind are the audit
# record: commit them, then tag that commit v${VERSION}.
pkg_all: clean build_all
	for a in ${ARCHES}; do \
		./ian set -a $$a || exit 1; \
		cp ./release/ian.$$a.Linux usr/bin/ian || exit 1; \
		./ian add usr/bin/ian || exit 1; \
		./ian pkg || exit 1; \
	done
	cp pkg/* release

# Rebuild every architecture and check the binaries against the committed sums,
# without touching them.  Run on a tagged checkout to prove the release is
# reproducible from the commit it was tagged on.
verify: clean build_all
	@tag=$$(git describe --tags --exact-match 2>/dev/null); \
	if [ -n "$$tag" ] && [ "$$tag" != "v${VERSION}" ]; then \
		echo "tag $$tag does not match DEBIAN/control Version ${VERSION}"; exit 1; \
	fi
	for a in ${ARCHES}; do \
		cp ./release/ian.$$a.Linux usr/bin/ian || exit 1; \
		grep ' usr/bin/ian$$' DEBIAN/md5sums.$$a | md5sum -c - || exit 1; \
	done

.PHONY: default amd64 check_version build_all build local release clean pkg_all verify
