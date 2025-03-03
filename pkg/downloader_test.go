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

type mockClient struct {
	do func(*http.Request) (*http.Response, error)
}

func (m mockClient) Do(req *http.Request) (*http.Response, error) {
	return m.do(req)
}

type mockIoer struct {
	mkdirAll func(string, os.FileMode) error
	create   func(string) (io.WriteCloser, error)
	stat     func(string) (os.FileInfo, error)
}

func (m mockIoer) MkdirAll(path string, perm os.FileMode) error {
	return m.mkdirAll(path, perm)
}

func (m mockIoer) Create(name string) (io.WriteCloser, error) {
	return m.create(name)
}

func (m mockIoer) Stat(name string) (os.FileInfo, error) {
	return m.stat(name)
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

var (
	//go:embed testdata/album_home.html
	home string
	//go:embed testdata/song1.html
	song1 string
)

func TestFetchAlbum(t *testing.T) {
	t.Run("title not found exit early", func(t *testing.T) {
		client := mockClient{
			do: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("<div></div>")),
				}, nil
			},
		}
		logger := slog.New(slog.DiscardHandler)

		_, err := FetchAlbum(context.Background(), client, nil, logger, ".", ".", false)
		if err == nil || !strings.Contains(err.Error(), "album name") {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("happy path with download", func(t *testing.T) {
		mp := map[string]string{
			"https://example.com/":                  home,
			"https://example.com/01.%2520song1.mp3": song1,
			"https://example.com/01.%2520song2.mp3": strings.ReplaceAll(strings.ReplaceAll(song1, "song1", "song2"), "01", "02"),
			"https://download.com/Cover.jpg":        "content of cover",
			"https://download.com/01.%20song1.flac": "content of song1",
			"https://download.com/02.%20song2.flac": "content of song2",
		}
		client := mockClient{
			do: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(mp[req.URL.String()])),
				}, nil
			},
		}

		fsRecord := map[string]string{}
		ioer := mockIoer{
			mkdirAll: func(path string, perm os.FileMode) error { return nil },
			create: func(name string) (io.WriteCloser, error) {
				fsRecord[name] = ""
				return &mockWriteCloser{
					write: func(p []byte) (n int, err error) {
						fsRecord[name] += string(p)
						return len(p), nil
					},
					close: func() error { return nil },
				}, nil
			},
			stat: func(name string) (os.FileInfo, error) { return nil, &os.PathError{} },
		}

		logger := slog.New(slog.DiscardHandler)

		res, err := FetchAlbum(context.Background(), client, ioer, logger, ".", "https://example.com/", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
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
		expDownloadedFiles := map[string]string{
			"My Album/Cover.jpg":      "content of cover",
			"My Album/01. song1.flac": "content of song1",
			"My Album/02. song2.flac": "content of song2",
		}
		for path, content := range expDownloadedFiles {
			if fsRecord[path] != content {
				t.Fatalf("expected %s to have content %s, got %s", path, content, fsRecord[path])
			}
		}

		for _, fn := range []string{"My Album/info.json", "My Album/page.url"} {
			if !strings.Contains(fsRecord[fn], "https://example.com/") {
				t.Fatalf("expected %s to be created", fn)
			}
		}
	})
}
