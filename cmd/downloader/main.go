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
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flag.Usage = cmd.PrintUsage
	urlFlag := flag.String("url", "", "URL to download")
	noDownloadImageFlag := flag.Bool("no-download-image", false, "Don't download images. Default: false")
	noDownloadTrackFlag := flag.Bool("no-download-track", false, "Don't download tracks. Default: false")
	noDownloadFlag := flag.Bool("no-download", false, "Combine no-download-image and no-download-track. Default: false")
	fixTags := flag.Bool("fix-tags", false, "Fix tags of the downloaded files. Default: false")
	overwriteFlag := flag.Bool("overwrite", false, "Redownload existing files. This option does not affect generation of info.json and link. Default: false")
	trackFlag := trackFlags{}
	flag.Var(&trackFlag, "track", "Tracks to download. Format: [disc number-]track number. Example: 1-1,1-2. Default to all tracks.")
	flag.Parse()
	if *urlFlag == "" {
		flag.Usage()
		logger.Error("url is required")
		os.Exit(1)
	}
	if *noDownloadFlag {
		*noDownloadImageFlag = true
		*noDownloadTrackFlag = true
	}
	if *noDownloadImageFlag && *noDownloadTrackFlag {
		*noDownloadFlag = true
	}
	if *noDownloadFlag && *overwriteFlag {
		logger.Warn("specifying overwrite while no-download is set has no effect")
	}
	if len(trackFlag) == 0 {
		trackFlag = nil
	} else if *noDownloadTrackFlag {
		logger.Warn("specifying track while no-download-track is set has no effect")
	}

	info, folder, err := pkg.FetchAlbum(context.Background(), http.DefaultClient, logger, ".", *urlFlag, *noDownloadImageFlag, *noDownloadTrackFlag, *overwriteFlag, pkg.TrackNumberSet(trackFlag))
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	if *fixTags {
		logger.Info("fixing tags")
		err := pkg.FixTags(logger, pkg.AlbumInfoToTags(info), pkg.AlbumInfoToFileTags(info), folder, false, false, false, false)
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
	}
}
