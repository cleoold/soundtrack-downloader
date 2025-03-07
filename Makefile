MAKEFLAGS += --always-make

build:
	go build -o bin/downloader ./cmd/downloader
	go build -o bin/meta ./cmd/meta

test:
	go test -v ./...
