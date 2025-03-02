package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/cleoold/soundtrack-downloader/pkg"
)

func main() {
	urlFlag := flag.String("url", "", "URL to download")
	flag.Parse()
	if *urlFlag == "" {
		flag.Usage()
		os.Exit(1)
	}
	_, err := pkg.FetchAlbum(context.Background(), *urlFlag)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
