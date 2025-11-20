VERSION=$(shell git describe --tags|tr -d 'v')
LDFLAGS=-ldflags "-X main.version=${VERSION}"

default: pkg_all

build_all:
	GOOS=linux GOARCH=arm go build ${LDFLAGS} -o ./release/ian.armhf.Linux ./cmd/ian
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o ./release/ian.arm64.Linux ./cmd/ian
	GOOS=linux GOARCH=arm GOARM=6 go build ${LDFLAGS} -o ./release/ian.armel.Linux ./cmd/ian
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ./release/ian.amd64.Linux ./cmd/ian
	GOOS=linux GOARCH=386 go build ${LDFLAGS} -o ./release/ian.i386.Linux ./cmd/ian

clean:
	rm -fr dpkg/pkg/*
	rm -fr release

pkg_all: clean build_all
	DEBUG=1 IAN_DIR=dpkg ./release/ian.amd64.Linux set -v ${VERSION}

	DEBUG=1 IAN_DIR=dpkg ./release/ian.amd64.Linux set -a i386
	cp ./release/ian.i386.Linux dpkg/usr/bin/ian
	DEBUG=1 IAN_DIR=dpkg ./release/ian.amd64.Linux pkg

	DEBUG=1 IAN_DIR=dpkg ./release/ian.amd64.Linux set -a armhf
	cp ./release/ian.armhf.Linux dpkg/usr/bin/ian
	DEBUG=1 IAN_DIR=dpkg ./release/ian.amd64.Linux pkg

	DEBUG=1 IAN_DIR=dpkg ./release/ian.amd64.Linux set -a amd64
	cp ./release/ian.amd64.Linux dpkg/usr/bin/ian
	DEBUG=1 IAN_DIR=dpkg ./release/ian.amd64.Linux pkg

	cp dpkg/pkg/* release
