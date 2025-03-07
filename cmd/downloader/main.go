package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/cleoold/soundtrack-downloader/cmd"
	"github.com/cleoold/soundtrack-downloader/pkg"
)

func main() {
	flag.Usage = cmd.PrintUsage
	urlFlag := flag.String("url", "", "URL to download")
	noDownloadFlag := flag.Bool("no-download", false, "Don't download the files. Default: false")
	fixTags := flag.Bool("fix-tags", false, "Fix tags of the downloaded files. Default: false")
	flag.Parse()
	if *urlFlag == "" {
		flag.Usage()
		slog.Error("url is required")
		os.Exit(1)
	}

	info, folder, err := pkg.FetchAlbum(context.Background(), http.DefaultClient, slog.Default(), ".", *urlFlag, *noDownloadFlag)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	if *fixTags {
		slog.Info("fixing tags")
		err := pkg.FixTags(slog.Default(), pkg.AlbumInfoToTags(info), folder, true, false, false, false)
		if err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
	}
}
