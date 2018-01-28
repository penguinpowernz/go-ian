release:
	GOOS=darwin GOARCH=amd64 go build -o ian.x86_64.Darwin ./cmd/ian
	GOOS=linux GOARCH=amd64 go build -o ian.x86_64.Linux ./cmd/ian