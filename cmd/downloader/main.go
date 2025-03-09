package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/cleoold/soundtrack-downloader/cmd"
	"github.com/cleoold/soundtrack-downloader/pkg"
)

type trackFlags pkg.TrackNumberSet

func (t trackFlags) String() string {
	return fmt.Sprintf("%v", pkg.TrackNumberSet(t))
}

func (t trackFlags) Set(value string) error {
	tt := pkg.TrackNumberSet(t)
	parts := strings.SplitSeq(value, ",")
	for part := range parts {
		part := strings.TrimSpace(part)
		// Note: no validation like convertibility to int is done here
		sp := strings.Split(part, "-")
		if len(sp) == 1 {
			tt.Add(pkg.TrackNumberKey{TrackNumber: sp[0]})
		} else if len(sp) == 2 {
			tt.Add(pkg.TrackNumberKey{DiscNumber: sp[0], TrackNumber: sp[1]})
		} else {
			return fmt.Errorf("invalid track number format: %s", value)
		}
	}
	return nil
}

func main() {
	flag.Usage = cmd.PrintUsage
	urlFlag := flag.String("url", "", "URL to download")
	noDownloadFlag := flag.Bool("no-download", false, "Don't download the files. Default: false")
	fixTags := flag.Bool("fix-tags", false, "Fix tags of the downloaded files. Default: false")
	overwriteFlag := flag.Bool("overwrite", false, "Redownload existing files. This option does not affect generation of info.json and link. Default: false")
	trackFlag := trackFlags{}
	flag.Var(&trackFlag, "track", "Tracks to download. Format: [disc number-]track number. Example: 1-1,1-2. Default to all tracks.")
	flag.Parse()
	if *urlFlag == "" {
		flag.Usage()
		slog.Error("url is required")
		os.Exit(1)
	}
	if *noDownloadFlag && *overwriteFlag {
		slog.Warn("specifying overwrite while nodownload is set has no effect")
	}
	if len(trackFlag) == 0 {
		trackFlag = nil
	}

	info, folder, err := pkg.FetchAlbum(context.Background(), http.DefaultClient, slog.Default(), ".", *urlFlag, *noDownloadFlag, *overwriteFlag, pkg.TrackNumberSet(trackFlag))
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
