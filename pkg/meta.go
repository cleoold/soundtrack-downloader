package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.senan.xyz/taglib"
)

var (
	// File name may be "1-01. Track Name.flac"
	discTrackNameRegex = regexp.MustCompile(`^(\d+)-(\d+)\.\s*(.+)\.([\w\d]+)$`)
	// File name may be "01. Track Name.flac"
	trackNameRegex = regexp.MustCompile(`^(\d+)\.\s*(.+)\.([\w\d]+)$`)
)

func inferTagsFromFileName(fileName string) map[string]string {
	res := map[string]string{}
	if match := discTrackNameRegex.FindStringSubmatch(fileName); match != nil {
		// there's also DiscTotal
		res[taglib.DiscNumber] = match[1]
		res[taglib.TrackNumber] = match[2]
		res[taglib.Title] = match[3]
	} else if match := trackNameRegex.FindStringSubmatch(fileName); match != nil {
		res[taglib.TrackNumber] = match[1]
		res[taglib.Title] = match[2]
	} else {
		res[taglib.Title] = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}
	return res
}

var musicExts = InsStringKeySet{"FLAC": {}, "MP3": {}, "MPC": {}, "OGG": {}, "M4A": {}, "MP4": {}, "WAV": {}, "AAC": {}}

func fixTags(
	logger *slog.Logger,
	osOpen func(name string) (io.ReadCloser, error),
	osReadDir func(name string) ([]os.DirEntry, error),
	taglibReadTags func(path string) (map[string][]string, error),
	taglibWriteTags func(path string, tags map[string][]string, opts taglib.WriteOption) error,
	tags map[string]string,
	fileSpecificTags map[string]map[string]string,
	overwrites TagKeySet,
	workpath string,
	inferNames bool,
	readAlbumInfo bool,
	noFix bool,
) error {
	// Precedence: supplied file-specific tags
	// > supplied tags
	// > inferred file-specific tags
	// > albumInfo file-specific tags
	// > albumInfo tags
	var albumInfoTags map[string]string
	var albumInfoFileSpecificTags map[string]map[string]string
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
		albumInfoTags = AlbumInfoToTags(&albumInfo)
		albumInfoFileSpecificTags = AlbumInfoToFileTags(&albumInfo)
	}

	dirEntries, err := osReadDir(workpath)
	if err != nil {
		return err
	}

	for _, dirEntry := range dirEntries {
		upperExt := strings.ToUpper(strings.TrimPrefix(filepath.Ext(dirEntry.Name()), "."))
		if dirEntry.IsDir() || !musicExts.Contains(upperExt) {
			continue
		}

		actualTags := map[string][]string{}
		stackTags := func(src map[string]string) {
			for k, v := range src {
				actualTags[k] = []string{v}
			}
		}

		stackTags(albumInfoTags)
		if fileTags, ok := albumInfoFileSpecificTags[dirEntry.Name()]; ok {
			stackTags(fileTags)
		}
		if inferNames {
			stackTags(inferTagsFromFileName(dirEntry.Name()))
		}
		stackTags(tags)
		if fileTags, ok := fileSpecificTags[dirEntry.Name()]; ok {
			stackTags(fileTags)
		}

		songPath := filepath.Join(workpath, dirEntry.Name())
		// Test validity
		existingTags, err := taglibReadTags(songPath)
		if err != nil {
			logger.Error(fmt.Sprintf("failed to read tags for %s: %s", dirEntry.Name(), err.Error()))
			continue
		}
		logger.Debug(fmt.Sprintf("existing tags for %s: %+v", dirEntry.Name(), existingTags))
		for k := range existingTags {
			if !overwrites.Contains(k) {
				delete(actualTags, k)
			}
		}

		if len(actualTags) == 0 {
			logger.Info(fmt.Sprintf("nothing to do for %s", dirEntry.Name()))
			continue
		}

		pairs := make([]string, 0, len(actualTags))
		for k, v := range actualTags {
			pairs = append(pairs, fmt.Sprintf("%s=\"%s\"", k, v[0]))
		}
		catPairs := strings.Join(pairs, ", ")
		if noFix {
			logger.Info(fmt.Sprintf("proposed %d tags for %s: %s", len(pairs), dirEntry.Name(), catPairs))
			continue
		}

		logger.Info(fmt.Sprintf("setting %d tags for %s: %v", len(pairs), dirEntry.Name(), catPairs))

		if err := taglibWriteTags(songPath, actualTags, 0); err != nil {
			logger.Error(fmt.Sprintf("failed to write tags for %s: %s", dirEntry.Name(), err.Error()))
		}
	}
	return nil
}

func FixTags(
	logger *slog.Logger,
	tags map[string]string,
	fileSpecificTags map[string]map[string]string,
	overwrites TagKeySet,
	workpath string,
	inferNames bool,
	readAlbumInfo bool,
	noFix bool,
) error {
	osOpen := func(name string) (io.ReadCloser, error) {
		return os.Open(name) // covariance
	}
	return fixTags(logger, osOpen, os.ReadDir, taglib.ReadTags, taglib.WriteTags, tags, fileSpecificTags, overwrites, workpath, inferNames, readAlbumInfo, noFix)
}

type TagKeySet = InsStringKeySet

var (
	NoOverWriteTags  = TagKeySet{}
	OverwriteAllTags = TagKeySet{"*": {}}
)

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
		tags[taglib.Label] = albumInfo.Publisher
		if albumInfo.Developer == "" {
			tags[taglib.Artist] = albumInfo.Publisher
			tags[taglib.AlbumArtist] = albumInfo.Publisher
		}
	}
	if albumInfo.CatalogNumber != "" {
		tags[taglib.CatalogNumber] = albumInfo.CatalogNumber
	}
	if albumInfo.AlbumType != "" {
		tags[taglib.Genre] = albumInfo.AlbumType
	}
	return tags
}

// Maps file names to tags
func AlbumInfoToFileTags(albumInfo *AlbumInfo) map[string]map[string]string {
	res := map[string]map[string]string{}
	for i := range albumInfo.Tracks {
		t := &albumInfo.Tracks[i]
		tags := map[string]string{}
		if t.Name != "" {
			tags[taglib.Title] = t.Name
		}
		if t.DiscNumber != "" {
			tags[taglib.DiscNumber] = t.DiscNumber
		}
		if t.TrackNumber != "" {
			tags[taglib.TrackNumber] = t.TrackNumber
		}
		// Get file name
		for _, link := range t.SongUrl {
			unescaped, _ := url.QueryUnescape(link)
			fileName := sanitizeFilename(path.Base(unescaped))
			res[fileName] = tags
		}
	}
	return res
}

type InsStringKeySet map[string]struct{}

func (s InsStringKeySet) Contains(v string) bool {
	if _, ok := s["*"]; ok {
		return true
	}
	_, ok := s[strings.ToUpper(v)]
	return ok
}

func (s InsStringKeySet) Add(v string) {
	s[strings.ToUpper(v)] = struct{}{}
}
