package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/cleoold/soundtrack-downloader/pkg"
)

type ioer struct{}

func (ioer) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (ioer) Create(name string) (io.WriteCloser, error)   { return os.Create(name) }
func (ioer) Stat(name string) (os.FileInfo, error)        { return os.Stat(name) }

func main() {
	urlFlag := flag.String("url", "", "URL to download")
	noDownloadFlag := flag.Bool("no-download", false, "Don't download the files")
	flag.Parse()
	_, err := pkg.FetchAlbum(context.Background(), http.DefaultClient, ioer{}, slog.Default(), ".", *urlFlag, *noDownloadFlag)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
