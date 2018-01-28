release:
	GOOS=darwin GOARCH=amd64 go build -o ian.x86_64.Darwin ./cmd/ian
	GOOS=linux GOARCH=amd64 go build -o ian.x86_64.Linux ./cmd/ian
	GOOS=linux GOARCH=arm go build -o ian.armhf.Linux ./cmd/ian
	GOOS=linux GOARCH=arm GOARM=6 go build -o ian.armel.Linux ./cmd/ian
