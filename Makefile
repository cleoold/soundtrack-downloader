MAKEFLAGS += --always-make

build-all: build-linux build-windows build-apple-silicon

build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/downloader ./cmd/downloader
	GOOS=linux GOARCH=amd64 go build -o bin/meta ./cmd/meta

build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/downloader.exe ./cmd/downloader
	GOOS=windows GOARCH=amd64 go build -o bin/meta.exe ./cmd/meta

build-apple-silicon:
	GOOS=darwin GOARCH=arm64 go build -o bin/downloader_arm64 ./cmd/downloader
	GOOS=darwin GOARCH=arm64 go build -o bin/meta_arm64 ./cmd/meta

test:
	go test -v ./...
