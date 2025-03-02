MAKEFLAGS += --always-make

build:
	go build -o bin/downloader ./cmd/downloader
