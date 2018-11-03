
release:
	go get -v ./...
	GOOS=darwin GOARCH=amd64 go build -o ian.x86_64.Darwin ./cmd/ian
	GOOS=linux GOARCH=arm go build -o ian.armhf.Linux ./cmd/ian
	GOOS=linux GOARCH=arm GOARM=6 go build -o ian.armel.Linux ./cmd/ian
	GOOS=linux GOARCH=amd64 go build -o ian.x86_64.Linux ./cmd/ian
	GOOS=linux GOARCH=386 go build -o ian.i386.Linux ./cmd/ian
	
	DEBUG=1 IAN_DIR=dpkg ./ian.x86_64.Linux set -a i386
	DEBUG=1 IAN_DIR=dpkg ./ian.x86_64.Linux bp
	DEBUG=1 IAN_DIR=dpkg ./ian.x86_64.Linux set -a armhf
	DEBUG=1 IAN_DIR=dpkg ./ian.x86_64.Linux bp
	DEBUG=1 IAN_DIR=dpkg ./ian.x86_64.Linux set -a amd64
	DEBUG=1 IAN_DIR=dpkg ./ian.x86_64.Linux bp
