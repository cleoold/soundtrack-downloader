package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"testing"

	"go.senan.xyz/taglib"
)

type mockDirEntry struct {
	name  string
	isDir bool
}

func (m *mockDirEntry) Info() (os.FileInfo, error) { return nil, nil }
func (m *mockDirEntry) Name() string               { return m.name }
func (m *mockDirEntry) IsDir() bool                { return m.isDir }
func (m *mockDirEntry) Type() os.FileMode          { return 0 }

func TestFixTags(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	t.Run("happy path merges provided tags, inferred triplets, album info and existing tags in order", func(t *testing.T) {
		providedtags := map[string]string{
			taglib.Artist:      "MyArtist",
			taglib.AlbumArtist: "MyAlbumArtist",
		}
		mkOpen := func(name string) (io.ReadCloser, error) {
			if name != "My Album/info.json" {
				t.Fatalf("expected to open info.json, got %s", name)
			}
			info := AlbumInfo{
				Name:      "MyAlbum",
				Year:      "2021",
				Developer: "MyDev",
			}
			buffer := new(bytes.Buffer)
			_ = json.NewEncoder(buffer).Encode(info)
			return io.NopCloser(buffer), nil
		}
		mkOsReadDir := func(name string) ([]os.DirEntry, error) {
			if name != "My Album" {
				t.Fatalf("expected to read My Album, got %s", name)
			}
			return []os.DirEntry{
				&mockDirEntry{name: "1-01. Song1.flac", isDir: false},
				&mockDirEntry{name: "1-02. Song2.flac", isDir: false},
				&mockDirEntry{name: "info.json", isDir: false},
			}, nil
		}
		existingTags := map[string]map[string][]string{
			"My Album/1-01. Song1.flac": {
				taglib.Artist: {"CrazyArtist"},
				taglib.Genre:  {"Rock"},
			},
			"My Album/1-02. Song2.flac": {
				taglib.Artist: {"CrazyArtist"},
				taglib.Genre:  {"Rock"},
			},
		}
		mkReadTags := func(path string) (map[string][]string, error) {
			return existingTags[path], nil
		}
		records := map[string]map[string][]string{}
		mkWriteTags := func(path string, tags map[string][]string, opts taglib.WriteOption) error {
			if opts != 0 {
				t.Fatalf("expected no options, got %v", opts)
			}
			records[path] = tags
			return nil
		}
		err := fixTags(logger, mkOpen, mkOsReadDir, mkReadTags, mkWriteTags, providedtags, nil, NoOverWriteTags, "My Album", true, true, false)
		if err != nil {
			t.Fatalf("unexpected error: %s", err.Error())
		}
		expectedRecords := map[string]map[string][]string{
			"My Album/1-01. Song1.flac": {
				taglib.AlbumArtist: {"MyAlbumArtist"},
				taglib.Album:       {"MyAlbum"},
				taglib.Date:        {"2021"},
				taglib.Title:       {"Song1"},
				taglib.DiscNumber:  {"1"},
				taglib.TrackNumber: {"01"},
			},
			"My Album/1-02. Song2.flac": {
				taglib.AlbumArtist: {"MyAlbumArtist"},
				taglib.Album:       {"MyAlbum"},
				taglib.Date:        {"2021"},
				taglib.Title:       {"Song2"},
				taglib.DiscNumber:  {"1"},
				taglib.TrackNumber: {"02"},
			},
		}
		if !reflect.DeepEqual(records, expectedRecords) {
			t.Fatalf("expected records to be %v, got %v", expectedRecords, records)
		}
	})

	t.Run("happy path merges provided tags, inferred doublets existing tags in order", func(t *testing.T) {
		providedtags := map[string]string{
			taglib.Artist:      "MyArtist",
			taglib.AlbumArtist: "MyAlbumArtist",
		}
		mkOsReadDir := func(name string) ([]os.DirEntry, error) {
			if name != "My Album" {
				t.Fatalf("expected to read My Album, got %s", name)
			}
			return []os.DirEntry{
				&mockDirEntry{name: "01. Song1 - Happy.flac", isDir: false},
				&mockDirEntry{name: "02. Song2 - Sad.flac", isDir: false},
				&mockDirEntry{name: "info.json", isDir: false},
			}, nil
		}
		existingTags := map[string]map[string][]string{
			"My Album/01. Song1 - Happy.flac": {
				taglib.Artist: {"CrazyArtist"},
				taglib.Genre:  {"Rock"},
				taglib.Title:  {"Song1 - Happy"},
			},
			"My Album/02. Song2 - Sad.flac": {
				taglib.Artist: {"CrazyArtist"},
				taglib.Genre:  {"Rock"},
			},
		}
		mkReadTags := func(path string) (map[string][]string, error) {
			return existingTags[path], nil
		}
		records := map[string]map[string][]string{}
		mkWriteTags := func(path string, tags map[string][]string, opts taglib.WriteOption) error {
			records[path] = tags
			return nil
		}
		err := fixTags(logger, nil, mkOsReadDir, mkReadTags, mkWriteTags, providedtags, nil, NoOverWriteTags, "My Album", true, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %s", err.Error())
		}
		expectedRecords := map[string]map[string][]string{
			"My Album/01. Song1 - Happy.flac": {
				taglib.AlbumArtist: {"MyAlbumArtist"},
				taglib.TrackNumber: {"01"},
			},
			"My Album/02. Song2 - Sad.flac": {
				taglib.AlbumArtist: {"MyAlbumArtist"},
				taglib.Title:       {"Song2 - Sad"},
				taglib.TrackNumber: {"02"},
			},
		}
		if !reflect.DeepEqual(records, expectedRecords) {
			t.Fatalf("expected records to be %v, got %v", expectedRecords, records)
		}
	})

	t.Run("happy path uses inferred names to overwrite existing tags", func(t *testing.T) {
		mkOsReadDir := func(name string) ([]os.DirEntry, error) {
			if name != "My Album" {
				t.Fatalf("expected to read My Album, got %s", name)
			}
			return []os.DirEntry{
				&mockDirEntry{name: "Song1 - Happy.mp3", isDir: false},
				&mockDirEntry{name: "invalid.mp3", isDir: false},
				&mockDirEntry{name: ".trash", isDir: true},
				&mockDirEntry{name: "Song2 - Sad.mp3", isDir: false},
			}, nil
		}
		existingTags := map[string]map[string][]string{
			"My Album/Song1 - Happy.mp3": {
				taglib.Artist: {"CrazyArtist"},
				taglib.Genre:  {"Rock"},
				taglib.Title:  {"Song1 - Happy"},
			},
			"My Album/Song2 - Sad.mp3": {
				taglib.Artist: {"CrazyArtist"},
				taglib.Genre:  {"Rock"},
			},
		}
		mkReadTags := func(path string) (map[string][]string, error) {
			if dat, ok := existingTags[path]; ok {
				return dat, nil
			}
			return nil, fmt.Errorf("no tags found")
		}
		records := map[string]map[string][]string{}
		mkWriteTags := func(path string, tags map[string][]string, opts taglib.WriteOption) error {
			records[path] = tags
			return nil
		}
		err := fixTags(logger, nil, mkOsReadDir, mkReadTags, mkWriteTags, nil, nil, OverwriteAllTags, "My Album", true, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %s", err.Error())
		}
		expectedRecords := map[string]map[string][]string{
			"My Album/Song1 - Happy.mp3": {
				taglib.Title: {"Song1 - Happy"},
			},
			"My Album/Song2 - Sad.mp3": {
				taglib.Title: {"Song2 - Sad"},
			},
		}
		if !reflect.DeepEqual(records, expectedRecords) {
			t.Fatalf("expected records to be %v, got %v", expectedRecords, records)
		}
	})

	t.Run("happy path merges provided file tags, provided tags, album info, and album info file tags in order", func(t *testing.T) {
		providedFileTags := map[string]map[string]string{
			"1-01. Song1.flac": {
				taglib.Artist:      "Solo",
				taglib.Title:       "Song 1 (Fixed)",
				taglib.DiscNumber:  "1",
				taglib.TrackNumber: "01",
			},
		}
		providedTags := map[string]string{
			taglib.Date:        "2022",
			taglib.AlbumArtist: "My Album Artist",
		}
		mkOpen := func(name string) (io.ReadCloser, error) {
			if name != "My Album/info.json" {
				t.Fatalf("expected to open info.json, got %s", name)
			}
			info := AlbumInfo{
				Name:      "My Album",
				Year:      "2021",
				Developer: "My Dev",
				Tracks: []TrackInfo{
					{
						SongUrl:     map[string]string{"FLAC": "https://example.com/1-01. Song1.flac"},
						Name:        "Song 1",
						DiscNumber:  "1",
						TrackNumber: "1",
					},
					{
						SongUrl:     map[string]string{"flac": "https://example.com/1-02.%20Song2.flac"},
						Name:        "Song 2",
						DiscNumber:  "1",
						TrackNumber: "2",
					},
				},
			}
			buffer := new(bytes.Buffer)
			_ = json.NewEncoder(buffer).Encode(info)
			return io.NopCloser(buffer), nil
		}
		mkOsReadDir := func(name string) ([]os.DirEntry, error) {
			if name != "My Album" {
				t.Fatalf("expected to read My Album, got %s", name)
			}
			return []os.DirEntry{
				&mockDirEntry{name: "1-01. Song1.flac"},
				&mockDirEntry{name: "1-02. Song2.flac"},
				&mockDirEntry{name: "info.json"},
			}, nil
		}
		mkReadTags := func(path string) (map[string][]string, error) { return nil, nil }
		records := map[string]map[string][]string{}
		mkWriteTags := func(path string, tags map[string][]string, opts taglib.WriteOption) error {
			records[path] = tags
			return nil
		}
		err := fixTags(logger, mkOpen, mkOsReadDir, mkReadTags, mkWriteTags, providedTags, providedFileTags, OverwriteAllTags, "My Album", false, true, false)
		if err != nil {
			t.Fatalf("unexpected error: %s", err.Error())
		}
		expectedRecords := map[string]map[string][]string{
			"My Album/1-01. Song1.flac": {
				taglib.AlbumArtist: {"My Album Artist"},
				taglib.Artist:      {"Solo"},
				taglib.Album:       {"My Album"},
				taglib.Date:        {"2022"},
				taglib.Title:       {"Song 1 (Fixed)"},
				taglib.DiscNumber:  {"1"},
				taglib.TrackNumber: {"01"},
			},
			"My Album/1-02. Song2.flac": {
				taglib.AlbumArtist: {"My Album Artist"},
				taglib.Artist:      {"My Dev"},
				taglib.Album:       {"My Album"},
				taglib.Date:        {"2022"},
				taglib.Title:       {"Song 2"},
				taglib.DiscNumber:  {"1"},
				taglib.TrackNumber: {"2"},
			},
		}
		if !reflect.DeepEqual(records, expectedRecords) {
			t.Fatalf("expected records to be %v, got %v", expectedRecords, records)
		}
	})

	t.Run("happy path only prints the proposed changes when noFix is true", func(t *testing.T) {
		providedtags := map[string]string{
			taglib.Artist:      "MyArtist",
			taglib.AlbumArtist: "MyAlbumArtist",
		}
		mkOsReadDir := func(name string) ([]os.DirEntry, error) {
			if name != "My Album" {
				t.Fatalf("expected to read My Album, got %s", name)
			}
			return []os.DirEntry{&mockDirEntry{name: "Song1 - Happy.mp3", isDir: false}}, nil
		}
		mkReadTags := func(path string) (map[string][]string, error) {
			return map[string][]string{}, nil
		}
		mkWriteTags := func(path string, tags map[string][]string, opts taglib.WriteOption) error {
			t.Fatalf("unexpected write to %s", path)
			return nil
		}
		err := fixTags(logger, nil, mkOsReadDir, mkReadTags, mkWriteTags, providedtags, nil, OverwriteAllTags, "My Album", true, false, true)
		if err != nil {
			t.Fatalf("unexpected error: %s", err.Error())
		}
	})
}

func TestAlbumInfoToTags(t *testing.T) {
	tests := []struct {
		name     string
		info     AlbumInfo
		expected map[string]string
	}{
		{
			name: "happy path converts AlbumInfo to tags",
			info: AlbumInfo{
				Name:          "MyAlbum",
				Year:          "2021",
				Developer:     "MyDev",
				Publisher:     "MyPub",
				CatalogNumber: "123",
				AlbumType:     "MyType",
			},
			expected: map[string]string{
				taglib.Album:         "MyAlbum",
				taglib.Date:          "2021",
				taglib.Artist:        "MyDev",
				taglib.AlbumArtist:   "MyDev",
				taglib.Label:         "MyPub",
				taglib.CatalogNumber: "123",
				taglib.Genre:         "MyType",
			},
		},
		{
			name:     "happy path converts AlbumInfo to tags with empty fields",
			info:     AlbumInfo{},
			expected: map[string]string{},
		},
		{
			name: "happy path set publisher as artist when developer is empty",
			info: AlbumInfo{
				Name:      "MyAlbum",
				Year:      "2021",
				Publisher: "MyPub",
			},
			expected: map[string]string{
				taglib.Album:       "MyAlbum",
				taglib.Date:        "2021",
				taglib.Artist:      "MyPub",
				taglib.AlbumArtist: "MyPub",
				taglib.Label:       "MyPub",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := AlbumInfoToTags(&tt.info)
			if !reflect.DeepEqual(tags, tt.expected) {
				t.Fatalf("expected tags to be %v, got %v", tt.expected, tags)
			}
		})
	}
}

func TestAlbumInfoToFileTags(t *testing.T) {
	tests := []struct {
		name     string
		info     AlbumInfo
		expected map[string]map[string]string
	}{
		{
			name: "happy path converts AlbumInfo to file tags",
			info: AlbumInfo{
				Tracks: []TrackInfo{
					{
						Name:        "Song1",
						DiscNumber:  "1",
						TrackNumber: "01",
						SongUrl: map[string]string{
							"FLAC": "https://example.com/1-01. Song1.flac",
						},
					},
					{
						Name:        "Song2",
						TrackNumber: "02",
						SongUrl: map[string]string{
							"FLAC": "https://example.com/this-is-that/you%20%28you%20know%29/1-01.%20My%20Song%20%28By%20You%29.flac",
							"MP3":  "https://example.com/this-is-that/you%20%28you%20know%29/1-01.%20My%20Song%20%28By%20You%29.mp3",
						},
					},
				},
			},
			expected: map[string]map[string]string{
				"1-01. Song1.flac": {
					taglib.Title:       "Song1",
					taglib.DiscNumber:  "1",
					taglib.TrackNumber: "01",
				},
				"1-01. My Song (By You).flac": {
					taglib.Title:       "Song2",
					taglib.TrackNumber: "02",
				},
				"1-01. My Song (By You).mp3": {
					taglib.Title:       "Song2",
					taglib.TrackNumber: "02",
				},
			},
		},
		{
			name:     "happy path converts AlbumInfo to file tags with empty fields",
			info:     AlbumInfo{Name: "MyAlbum"},
			expected: map[string]map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := AlbumInfoToFileTags(&tt.info)
			if !reflect.DeepEqual(tags, tt.expected) {
				t.Fatalf("expected tags to be %v, got %v", tt.expected, tags)
			}
		})
	}
}

func TestInsStringKeySet(t *testing.T) {
	t.Run("Empty set", func(t *testing.T) {
		s := InsStringKeySet{}
		if k := "anything"; s.Contains(k) {
			t.Fatalf("expected set to not contain %s", k)
		}
	})

	t.Run("Contains all", func(t *testing.T) {
		s := InsStringKeySet{}
		s.Add("*")
		if k := "anything"; !s.Contains(k) {
			t.Fatalf("expected set to contain %s", k)
		}
	})

	t.Run("Contains specific", func(t *testing.T) {
		s := InsStringKeySet{}
		s.Add("album")
		if k := "ALBUM"; !s.Contains(k) {
			t.Fatalf("expected set to contain %s", k)
		}
		if k := "ARTIST"; s.Contains(k) {
			t.Fatalf("expected set to not contain %s", k)
		}
	})
}
