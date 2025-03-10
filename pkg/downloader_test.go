package pkg

import (
	"context"
	_ "embed"
	"io"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

type stubClient map[string]map[string]struct {
	code    int
	content string
}

func (m stubClient) Do(req *http.Request) (*http.Response, error) {
	pair := m[req.URL.String()][req.Method]
	return &http.Response{
		StatusCode: pair.code,
		Body:       io.NopCloser(strings.NewReader(pair.content)),
	}, nil
}

type mockWriteCloser struct {
	write func([]byte) (int, error)
	close func() error
}

func (m mockWriteCloser) Write(p []byte) (int, error) {
	return m.write(p)
}

func (m mockWriteCloser) Close() error {
	return m.close()
}

type FSRecorder map[string]string

func (m FSRecorder) Create(name string) (io.WriteCloser, error) {
	m[name] = ""
	return &mockWriteCloser{
		write: func(p []byte) (n int, err error) {
			m[name] += string(p)
			return len(p), nil
		},
		close: func() error { return nil },
	}, nil
}

var (
	//go:embed testdata/album1_home.html
	home1 string
	//go:embed testdata/album2_home.html
	home2 string
	//go:embed testdata/song1.html
	song1 string
)

func TestFetchAlbum(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	t.Run("title not found exit early", func(t *testing.T) {
		client := stubClient{
			".": {"GET": {http.StatusOK, "<div></div>"}},
		}
		_, _, err := fetchAlbum(context.Background(), client, logger, nil, nil, nil, ".", ".", false, false, false, false, false, DownloadAllTracks)
		if err == nil || !strings.Contains(err.Error(), "album name") {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("happy path with download and overwrites existing file", func(t *testing.T) {
		client := stubClient{
			"https://example.com/":                  {"GET": {http.StatusOK, home1}},
			"https://example.com/01.%2520song1.mp3": {"GET": {http.StatusOK, song1}},
			"https://example.com/01.%2520song2.mp3": {"GET": {http.StatusOK, strings.ReplaceAll(strings.ReplaceAll(song1, "song1", "song2"), "01", "02")}},
			"https://download.com/Cover.jpg":        {"GET": {http.StatusOK, "content of cover"}},
			"https://download.com/01.%20song1.flac": {"GET": {http.StatusOK, "content of song1"}},
			"https://download.com/02.%20song2.flac": {"GET": {http.StatusOK, "content of song2"}},
		}

		mkMkdirAll := func(path string, perm os.FileMode) error { return nil }
		mkFS := FSRecorder{}
		mkStat := func(name string) (os.FileInfo, error) {
			if name == "My Album 1/01. song1.flac" {
				return nil, nil
			}
			return nil, os.ErrNotExist
		}

		res, folder, err := fetchAlbum(context.Background(), client, logger, mkMkdirAll, mkFS.Create, mkStat, ".", "https://example.com/", false, false, false, false, true, DownloadAllTracks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if folder != "My Album 1" {
			t.Fatalf("expected folder to be My Album 1, got %s", folder)
		}
		expAlbumInfo := AlbumInfo{
			Url:       "https://example.com/",
			Name:      "My Album 1",
			Platforms: "MacOS; Windows",
			Year:      "2002",
			Developer: "My Studio; Other Studio",
			Publisher: "My Publisher",
			AlbumType: "Soundtrack",
			Images: []ImageInfo{
				{
					ImageUrl: "https://download.com/Cover.jpg",
					ThumbUrl: "https://download.com/thumbs/Cover.jpg",
				},
			},
			Tracks: []TrackInfo{
				{
					Name:        "song1",
					TrackNumber: "1",
					PageUrl:     "https://example.com/01.%2520song1.mp3",
					SongUrl:     "https://download.com/01.%20song1.flac",
				},
				{
					Name:        "song2",
					TrackNumber: "2",
					PageUrl:     "https://example.com/01.%2520song2.mp3",
					SongUrl:     "https://download.com/02.%20song2.flac",
				},
			},
		}
		if !reflect.DeepEqual(*res, expAlbumInfo) {
			t.Fatalf("expected %v, got %v", expAlbumInfo, *res)
		}

		// Check the downloaded files
		expFileCount := 5
		if len(mkFS) != expFileCount {
			t.Fatalf("expected %d files to be created, got %d", expFileCount, len(mkFS))
		}
		expDownloadedFiles := map[string]string{
			"My Album 1/Cover.jpg":      "content of cover",
			"My Album 1/01. song1.flac": "content of song1",
			"My Album 1/02. song2.flac": "content of song2",
		}
		for path, content := range expDownloadedFiles {
			if mkFS[path] != content {
				t.Fatalf("expected %s to have content %s, got %s", path, content, mkFS[path])
			}
		}

		for _, fn := range []string{"My Album 1/info.json", "My Album 1/page.url"} {
			if !strings.Contains(mkFS[fn], "https://example.com/") {
				t.Fatalf("expected %s to be created", fn)
			}
		}
	})

	// Add CD number
	song1CD := strings.ReplaceAll(song1, "01.%20song1", "1-01.%20song1")

	t.Run("happy path with download skips existing image, fails to download a track and skips link file", func(t *testing.T) {
		client := stubClient{
			"https://example.com/":                    {"GET": {http.StatusOK, home2}},
			"https://example.com/1-01.%2520song1.mp3": {"GET": {http.StatusOK, song1CD}},
			"https://example.com/1-01.%2520song2.mp3": {"GET": {http.StatusOK, strings.ReplaceAll(strings.ReplaceAll(song1CD, "song1", "song2"), "01", "02")}},
			"https://download.com/Cover.jpg":          {"GET": {http.StatusOK, "content of cover"}},
			"https://download.com/1-01.%20song1.flac": {"GET": {http.StatusNotFound, ""}},
			"https://download.com/1-02.%20song2.flac": {"GET": {http.StatusOK, "content of song2"}},
		}

		mkMkdirAll := func(path string, perm os.FileMode) error { return nil }
		mkFS := FSRecorder{}
		mkStat := func(name string) (os.FileInfo, error) {
			if name == "My Album 2/Cover.jpg" {
				return nil, nil
			}
			return nil, os.ErrNotExist
		}

		res, _, err := fetchAlbum(context.Background(), client, logger, mkMkdirAll, mkFS.Create, mkStat, ".", "https://example.com/", false, false, false, true, false, DownloadAllTracks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expAlbumInfo := AlbumInfo{
			Url:           "https://example.com/",
			Name:          "My Album 2",
			Platforms:     "MacOS; Windows",
			Year:          "2002",
			CatalogNumber: "ABC-2061~3",
			Publisher:     "My Publisher",
			AlbumType:     "Arrangement",
			Images: []ImageInfo{
				{
					ImageUrl: "https://download.com/Cover.jpg",
					ThumbUrl: "https://download.com/thumbs/Cover.jpg",
				},
			},
			Tracks: []TrackInfo{
				{
					Name:        "song1",
					DiscNumber:  "1",
					TrackNumber: "1",
					PageUrl:     "https://example.com/1-01.%2520song1.mp3",
					SongUrl:     "https://download.com/1-01.%20song1.flac",
				},
				{
					Name:        "song2",
					DiscNumber:  "1",
					TrackNumber: "2",
					PageUrl:     "https://example.com/1-01.%2520song2.mp3",
					SongUrl:     "https://download.com/1-02.%20song2.flac",
				}},
		}
		if !reflect.DeepEqual(*res, expAlbumInfo) {
			t.Fatalf("expected %v, got %v", expAlbumInfo, *res)
		}

		// Check the downloaded files
		if expFileCount := 2; len(mkFS) != expFileCount {
			t.Fatalf("expected %d files to be created, got %d", expFileCount, len(mkFS))
		}
		expDownloadedFiles := map[string]string{
			"My Album 2/1-02. song2.flac": "content of song2",
		}
		for path, content := range expDownloadedFiles {
			if mkFS[path] != content {
				t.Fatalf("expected %s to have content %s, got %s", path, content, mkFS[path])
			}
		}

		for _, fn := range []string{"My Album 2/info.json"} {
			if !strings.Contains(mkFS[fn], "https://example.com/") {
				t.Fatalf("expected %s to be created", fn)
			}
		}
	})

	t.Run("happy path only downloads songs in the set, does not download images and does not create info or link", func(t *testing.T) {
		client := stubClient{
			"https://example.com/":                    {"GET": {http.StatusOK, home2}},
			"https://example.com/1-01.%2520song1.mp3": {"GET": {http.StatusOK, song1CD}},
			"https://example.com/1-01.%2520song2.mp3": {"GET": {http.StatusOK, strings.ReplaceAll(strings.ReplaceAll(song1CD, "song1", "song2"), "01", "02")}},
			"https://download.com/Cover.jpg":          {"GET": {http.StatusOK, "content of cover"}},
			"https://download.com/1-01.%20song1.flac": {"GET": {http.StatusOK, "content of song1"}},
			"https://download.com/1-02.%20song2.flac": {"GET": {http.StatusOK, "content of song2"}},
		}

		mkMkdirAll := func(path string, perm os.FileMode) error { return nil }
		mkFS := FSRecorder{}
		mkStat := func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }

		set := TrackNumberSet{}
		set.Add(TrackNumberKey{"01", "002"})

		_, _, err := fetchAlbum(context.Background(), client, logger, mkMkdirAll, mkFS.Create, mkStat, ".", "https://example.com/", true, false, true, true, true, set)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check the downloaded files
		if expFileCount := 1; len(mkFS) != expFileCount {
			t.Fatalf("expected %d files to be created, got %d", expFileCount, len(mkFS))
		}
		expDownloadedFiles := map[string]string{
			"My Album 2/1-02. song2.flac": "content of song2",
		}
		for path, content := range expDownloadedFiles {
			if mkFS[path] != content {
				t.Fatalf("expected %s to have content %s, got %s", path, content, mkFS[path])
			}
		}
	})
}

func TestTrackNumberSet(t *testing.T) {
	t.Run("Empty set", func(t *testing.T) {
		s := TrackNumberSet{}
		if k := (&TrackInfo{DiscNumber: "1", TrackNumber: "1"}); s.Contains(k) {
			t.Fatalf("expected set to be empty")
		}
	})

	t.Run("wildcard-wildcard", func(t *testing.T) {
		s := TrackNumberSet{}
		s.Add(TrackNumberKey{"*", "*"})
		if k := (&TrackInfo{DiscNumber: "1", TrackNumber: "1"}); !s.Contains(k) {
			t.Fatalf("expected set to contain %v", k)
		}
	})

	t.Run("disc-wildcard", func(t *testing.T) {
		s := TrackNumberSet{}
		s.Add(TrackNumberKey{"01", "*"})
		if k := (&TrackInfo{DiscNumber: "1", TrackNumber: "1"}); !s.Contains(k) {
			t.Fatalf("expected set to contain %v", k)
		}
	})

	t.Run("wildcard-track", func(t *testing.T) {
		s := TrackNumberSet{}
		s.Add(TrackNumberKey{"*", "1"})
		if k := (&TrackInfo{DiscNumber: "1", TrackNumber: "01"}); !s.Contains(k) {
			t.Fatalf("expected set to contain %v", k)
		}
	})

	t.Run("disc-track", func(t *testing.T) {
		s := TrackNumberSet{}
		s.Add(TrackNumberKey{"1", "1"})
		s.Add(TrackNumberKey{"1", "3"})
		if k := (&TrackInfo{DiscNumber: "01", TrackNumber: "01"}); !s.Contains(k) {
			t.Fatalf("expected set to contain %v", k)
		}
		if k := (&TrackInfo{DiscNumber: "1", TrackNumber: "2"}); s.Contains(k) {
			t.Fatalf("expected set to not contain %v", k)
		}
	})
}
