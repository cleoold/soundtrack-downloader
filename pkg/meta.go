package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.senan.xyz/taglib"
)

var (
	// File name may be "1-01. Track Name.flac"
	discTrackNameRegex = regexp.MustCompile(`^(\d+)-(\d+)\.\s*(.+)\.(mp3|flac)$`)
	// File name may be "01. Track Name.flac"
	trackNameRegex = regexp.MustCompile(`^(\d+)\.\s*(.+)\.(mp3|flac)$`)
)

func fixTags(
	logger *slog.Logger,
	osOpen func(name string) (io.ReadCloser, error),
	osReadDir func(name string) ([]os.DirEntry, error),
	taglibReadTags func(path string) (map[string][]string, error),
	taglibWriteTags func(path string, tags map[string][]string, opts taglib.WriteOption) error,
	tags map[string]string,
	workpath string,
	inferNames,
	overwrite,
	readAlbumInfo bool,
) error {
	if readAlbumInfo {
		var albumInfo AlbumInfo
		f, err := osOpen(filepath.Join(workpath, "info.json"))
		if err != nil {
			return err
		}
		defer f.Close()
		if err := json.NewDecoder(f).Decode(&albumInfo); err != nil {
			return err
		}
		atags := AlbumInfoToTags(&albumInfo)
		maps.Copy(atags, tags)
		tags = atags
	}

	dirEntries, err := osReadDir(workpath)
	if err != nil {
		return err
	}

	for _, dirEntry := range dirEntries {
		ext := filepath.Ext(dirEntry.Name())
		if dirEntry.IsDir() || (ext != ".mp3" && ext != ".flac") {
			continue
		}

		// Obtain numbers and title
		actualTags := map[string][]string{}
		if inferNames {
			if match := discTrackNameRegex.FindStringSubmatch(dirEntry.Name()); match != nil {
				// there's also DiscTotal
				actualTags[taglib.DiscNumber] = []string{match[1]}
				actualTags[taglib.TrackNumber] = []string{match[2]}
				actualTags[taglib.Title] = []string{match[3]}
			} else if match := trackNameRegex.FindStringSubmatch(dirEntry.Name()); match != nil {
				actualTags[taglib.TrackNumber] = []string{match[1]}
				actualTags[taglib.Title] = []string{match[2]}
			} else {
				actualTags[taglib.Title] = []string{strings.TrimSuffix(dirEntry.Name(), ext)}
			}
		}
		for k, v := range tags {
			actualTags[k] = []string{v}
		}

		songPath := filepath.Join(workpath, dirEntry.Name())
		// Test validity
		existingTags, err := taglibReadTags(songPath)
		if err != nil {
			logger.Error(fmt.Sprintf("failed to read tags for %s: %s", dirEntry.Name(), err.Error()))
			continue
		}
		logger.Debug(fmt.Sprintf("existing tags for %s: %+v", dirEntry.Name(), existingTags))
		if !overwrite {
			for k := range existingTags {
				delete(actualTags, k)
			}
		}

		if len(actualTags) == 0 {
			logger.Info(fmt.Sprintf("nothing to do for %s", dirEntry.Name()))
			continue
		}

		keys := make([]string, 0, len(actualTags))
		for k := range actualTags {
			keys = append(keys, k)
		}
		logger.Info(fmt.Sprintf("setting %d tags for %s: %s", len(actualTags), dirEntry.Name(), strings.Join(keys, ", ")))

		if err := taglibWriteTags(songPath, actualTags, 0); err != nil {
			logger.Error(fmt.Sprintf("failed to write tags for %s: %s", dirEntry.Name(), err.Error()))
		}
	}
	return nil
}

func FixTags(logger *slog.Logger, tags map[string]string, workpath string, inferNames, overwrite, readAlbumInfo bool) error {
	osOpen := func(name string) (io.ReadCloser, error) {
		return os.Open(name) // covariance
	}
	return fixTags(logger, osOpen, os.ReadDir, taglib.ReadTags, taglib.WriteTags, tags, workpath, inferNames, overwrite, readAlbumInfo)
}

func AlbumInfoToTags(albumInfo *AlbumInfo) map[string]string {
	tags := map[string]string{}
	if albumInfo.Name != "" {
		tags[taglib.Album] = albumInfo.Name
	}
	if albumInfo.Year != "" {
		tags[taglib.Date] = albumInfo.Year
	}
	if albumInfo.Developer != "" {
		tags[taglib.Artist] = albumInfo.Developer
		tags[taglib.AlbumArtist] = albumInfo.Developer
	}
	if albumInfo.Publisher != "" {
		tags["PUBLISHER"] = albumInfo.Publisher
	}
	if albumInfo.AlbumType != "" {
		tags[taglib.Genre] = albumInfo.AlbumType
	}
	return tags
}
