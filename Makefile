VERSION=$(shell git describe --tags --always --dirty|tr -d 'v')
LDFLAGS=-ldflags "-X main.version=${VERSION}"

# the architectures a release is built for, and the suffix of the binary that
# build_all produces for each one
ARCHES=i386 armhf amd64 arm64 armel

default: build

amd64: local
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ./release/ian.amd64.Linux ./cmd/ian
	./ian set -a amd64
	cp ./release/ian.amd64.Linux usr/bin/ian
	./ian add usr/bin/ian
	./ian set -V
	./ian pkg

build_all:
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
	go build ./cmd/ian

release: local pkg_all

clean:
	rm -fr pkg/*
	rm -fr release

# Build every architecture, registering each binary against that arch's own
# manifest (DEBIAN/md5sums.<arch>) so every build verifies strictly against the
# sums committed for it.  The md5sums.* files this leaves behind are the audit
# record and should be committed with the release.
pkg_all: clean build_all
	./ian set -V
	for a in ${ARCHES}; do \
		./ian set -a $$a || exit 1; \
		cp ./release/ian.$$a.Linux usr/bin/ian || exit 1; \
		./ian add usr/bin/ian || exit 1; \
		./ian pkg || exit 1; \
	done
	cp pkg/* release
