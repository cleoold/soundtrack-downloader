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
	//go:embed testdata/album_home.html
	home string
	//go:embed testdata/song1.html
	song1 string
)

func TestFetchAlbum(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	t.Run("title not found exit early", func(t *testing.T) {
		client := stubClient{
			".": {"GET": {http.StatusOK, "<div></div>"}},
		}
		_, _, err := fetchAlbum(context.Background(), client, logger, nil, nil, nil, ".", ".", false, false)
		if err == nil || !strings.Contains(err.Error(), "album name") {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("happy path with download and overwrites existing file", func(t *testing.T) {
		client := stubClient{
			"https://example.com/":                  {"GET": {http.StatusOK, home}},
			"https://example.com/01.%2520song1.mp3": {"GET": {http.StatusOK, song1}},
			"https://example.com/01.%2520song2.mp3": {"GET": {http.StatusOK, strings.ReplaceAll(strings.ReplaceAll(song1, "song1", "song2"), "01", "02")}},
			"https://download.com/Cover.jpg":        {"GET": {http.StatusOK, "content of cover"}},
			"https://download.com/01.%20song1.flac": {"GET": {http.StatusOK, "content of song1"}},
			"https://download.com/02.%20song2.flac": {"GET": {http.StatusOK, "content of song2"}},
		}

		mkMkdirAll := func(path string, perm os.FileMode) error { return nil }
		mkFS := FSRecorder{}
		mkStat := func(name string) (os.FileInfo, error) {
			if name == "My Album/01. song1.flac" {
				return nil, nil
			}
			return nil, os.ErrNotExist
		}

		res, folder, err := fetchAlbum(context.Background(), client, logger, mkMkdirAll, mkFS.Create, mkStat, ".", "https://example.com/", false, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if folder != "My Album" {
			t.Fatalf("expected folder to be My Album, got %s", folder)
		}
		expAlbumInfo := AlbumInfo{
			Url:       "https://example.com/",
			Name:      "My Album",
			Platforms: "MacOS; Windows",
			Year:      "2002",
			Developer: "My Studio; Other Studio",
			Publisher: "My Publisher",
			AlbumType: "Soundtrack",
			ImageUrls: []string{"https://download.com/Cover.jpg"},
			Tracks: []TrackInfo{
				TrackInfo{PageUrl: "https://example.com/01.%2520song1.mp3", SongUrl: "https://download.com/01.%20song1.flac"},
				TrackInfo{PageUrl: "https://example.com/01.%2520song2.mp3", SongUrl: "https://download.com/02.%20song2.flac"}},
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
			"My Album/Cover.jpg":      "content of cover",
			"My Album/01. song1.flac": "content of song1",
			"My Album/02. song2.flac": "content of song2",
		}
		for path, content := range expDownloadedFiles {
			if mkFS[path] != content {
				t.Fatalf("expected %s to have content %s, got %s", path, content, mkFS[path])
			}
		}

		for _, fn := range []string{"My Album/info.json", "My Album/page.url"} {
			if !strings.Contains(mkFS[fn], "https://example.com/") {
				t.Fatalf("expected %s to be created", fn)
			}
		}
	})

	t.Run("happy path with download skips existing image and fails to download track", func(t *testing.T) {
		client := stubClient{
			"https://example.com/":                  {"GET": {http.StatusOK, home}},
			"https://example.com/01.%2520song1.mp3": {"GET": {http.StatusOK, song1}},
			"https://example.com/01.%2520song2.mp3": {"GET": {http.StatusOK, strings.ReplaceAll(strings.ReplaceAll(song1, "song1", "song2"), "01", "02")}},
			"https://download.com/Cover.jpg":        {"GET": {http.StatusOK, "content of cover"}},
			"https://download.com/01.%20song1.flac": {"GET": {http.StatusNotFound, ""}},
			"https://download.com/02.%20song2.flac": {"GET": {http.StatusOK, "content of song2"}},
		}

		mkMkdirAll := func(path string, perm os.FileMode) error { return nil }
		mkFS := FSRecorder{}
		mkStat := func(name string) (os.FileInfo, error) {
			if name == "My Album/Cover.jpg" {
				return nil, nil
			}
			return nil, os.ErrNotExist
		}

		_, _, err := fetchAlbum(context.Background(), client, logger, mkMkdirAll, mkFS.Create, mkStat, ".", "https://example.com/", false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check the downloaded files
		expFileCount := 3
		if len(mkFS) != expFileCount {
			t.Fatalf("expected %d files to be created, got %d", expFileCount, len(mkFS))
		}
		expDownloadedFiles := map[string]string{
			"My Album/02. song2.flac": "content of song2",
		}
		for path, content := range expDownloadedFiles {
			if mkFS[path] != content {
				t.Fatalf("expected %s to have content %s, got %s", path, content, mkFS[path])
			}
		}

		for _, fn := range []string{"My Album/info.json", "My Album/page.url"} {
			if !strings.Contains(mkFS[fn], "https://example.com/") {
				t.Fatalf("expected %s to be created", fn)
			}
		}
	})
}
