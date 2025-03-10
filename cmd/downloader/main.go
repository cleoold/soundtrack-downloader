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
	for part := range strings.SplitSeq(value, ",") {
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

type formatPreferenceFlags pkg.TrackFormatRanking

func (f *formatPreferenceFlags) String() string {
	return fmt.Sprintf("%v", pkg.TrackFormatRanking(*f))
}

func (f *formatPreferenceFlags) Set(value string) error {
	for part := range strings.SplitSeq(value, ",") {
		*f = append(*f, strings.ToUpper(strings.TrimSpace(part)))
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
	noCreateAlbumInfoFlag := flag.Bool("no-create-album-info", false, "Don't create info.json. Default: false")
	noCreateWindowsShortcutFlag := flag.Bool("no-create-windows-shortcut", false, "Don't create Windows shortcut. Default: false")
	fixTags := flag.Bool("fix-tags", false, "Fix tags of the downloaded files. Default: false")
	overwriteFlag := flag.Bool("overwrite", false, "Redownload existing files. This option does not affect generation of info.json and link. Default: false")
	trackFlag := trackFlags{}
	flag.Var(&trackFlag, "track", "Tracks to download. Format: [disc number-]track number. Example: -track 1-1,1-2. Special value '*' means all tracks. Default to all tracks.")
	trackFormatPreferenceFlag := formatPreferenceFlags{}
	flag.Var(&trackFormatPreferenceFlag, "track-format-preference", "File format preference. If available, files with types in the left of this list will be downloaded. Default to 'FLAC,MP3,OGG,*'")
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
	if *noDownloadFlag && *noCreateAlbumInfoFlag {
		logger.Warn("specifying both no-download and no-create-album-info results in nothing meaningful to do!")
	}
	if len(trackFlag) == 0 {
		trackFlag = trackFlags(pkg.DownloadAllTracks)
	} else if *noDownloadTrackFlag {
		logger.Warn("specifying track while no-download-track is set has no effect")
	}
	if len(trackFormatPreferenceFlag) == 0 {
		default_ := pkg.TrackFormatRanking{"FLAC", "MP3", "OGG", "*"}
		trackFormatPreferenceFlag = formatPreferenceFlags(default_)
	} else if *noDownloadTrackFlag {
		logger.Warn("specifying track-format-preference while no-download-track is set has no effect")
	}

	info, folder, err := pkg.FetchAlbum(context.Background(), http.DefaultClient, logger, ".", *urlFlag, *noDownloadImageFlag, *noDownloadTrackFlag, *noCreateAlbumInfoFlag, *noCreateWindowsShortcutFlag, *overwriteFlag, pkg.TrackNumberSet(trackFlag), pkg.TrackFormatRanking(trackFormatPreferenceFlag))
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	if *fixTags {
		logger.Info("fixing tags")
		err := pkg.FixTags(logger, pkg.AlbumInfoToTags(info), pkg.AlbumInfoToFileTags(info), nil, folder, false, false, false)
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
	}
}
