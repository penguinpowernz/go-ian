VERSION=$(shell git describe --tags|tr -d 'v')
LDFLAGS=-ldflags "-X main.version=${VERSION}"

default: build


build_all:
	GOOS=linux GOARCH=arm GOARM=7 go build ${LDFLAGS} -o ./release/ian.armhf.Linux ./cmd/ian
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o ./release/ian.arm64.Linux ./cmd/ian
	GOOS=linux GOARCH=arm GOARM=6 go build ${LDFLAGS} -o ./release/ian.armel.Linux ./cmd/ian
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ./release/ian.amd64.Linux ./cmd/ian
	GOOS=linux GOARCH=386 go build ${LDFLAGS} -o ./release/ian.i386.Linux ./cmd/ian

build: local
local:
	go build ./cmd/ian

release: local pkg_all

clean:
	rm -fr dpkg/pkg/*
	rm -fr release

pkg_all: clean build_all
	DEBUG=1 IAN_DIR=dpkg ./ian set -v ${VERSION}

	DEBUG=1 IAN_DIR=dpkg ./ian set -a i386
	cp ./release/ian.i386.Linux dpkg/usr/bin/ian
	DEBUG=1 IAN_DIR=dpkg ./ian pkg

	DEBUG=1 IAN_DIR=dpkg ./ian set -a armhf
	cp ./release/ian.armhf.Linux dpkg/usr/bin/ian
	DEBUG=1 IAN_DIR=dpkg ./ian pkg

	DEBUG=1 IAN_DIR=dpkg ./ian set -a amd64
	cp ./release/ian.amd64.Linux dpkg/usr/bin/ian
	DEBUG=1 IAN_DIR=dpkg ./ian pkg

	DEBUG=1 IAN_DIR=dpkg ./ian set -a arm64
	cp ./release/ian.arm64.Linux dpkg/usr/bin/ian
	DEBUG=1 IAN_DIR=dpkg ./ian pkg

	DEBUG=1 IAN_DIR=dpkg ./ian set -a armel
	cp ./release/ian.armel.Linux dpkg/usr/bin/ian
	DEBUG=1 IAN_DIR=dpkg ./ian pkg

	cp dpkg/pkg/* release
