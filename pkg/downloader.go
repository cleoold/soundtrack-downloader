package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	URL "net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func sanitizeFilename(filename string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, filename)
}

func joinUrl(home, link string) (string, error) {
	if strings.HasPrefix(link, "http") {
		return link, nil
	}
	u, err := URL.Parse(home)
	if err != nil {
		return "", err
	}
	u.Path = ""
	return u.JoinPath(link).String(), nil
}

type HttpDoClient interface {
	Do(*http.Request) (*http.Response, error)
}

func getUrl(ctx context.Context, client HttpDoClient, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected response: %d", resp.StatusCode)
	}
	return resp.Body, nil
}

var (
	platformRegex  = regexp.MustCompile(`(?m)Platforms:\s*(.+?)\s*$`)
	yearRegex      = regexp.MustCompile(`Year:\s*(\d+)`)
	developerRegex = regexp.MustCompile(`(?m)Developed by:\s*(.+?)\s*$`)
	publisherRegex = regexp.MustCompile(`(?m)Published by:\s*(.+?)\s*$`)
	albumTypeRegex = regexp.MustCompile(`(?m)Album type:\s*(.+?)\s*$`)
)

func FetchAlbumInfo(ctx context.Context, httpClient HttpDoClient, albumUrl string) (*AlbumInfo, error) {
	body, err := getUrl(ctx, httpClient, albumUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch album page: %w", err)
	}
	defer body.Close()
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse html file for album: %w", err)
	}

	result := AlbumInfo{Url: albumUrl}

	doc.Find("#pageContent h2").First().Each(func(i int, s *goquery.Selection) {
		result.Name = strings.TrimSpace(s.Text())
	})
	if result.Name == "" {
		return nil, fmt.Errorf("failed to find album name")
	}

	// Parse album info from description below title
	doc.Find("#pageContent p:contains('Platforms:')").First().Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if match := platformRegex.FindStringSubmatch(text); len(match) > 1 {
			result.Platforms = strings.ReplaceAll(match[1], ", ", "; ")
		}
		if match := yearRegex.FindStringSubmatch(text); len(match) > 1 {
			result.Year = match[1]
		}
		if match := developerRegex.FindStringSubmatch(text); len(match) > 1 {
			result.Developer = strings.ReplaceAll(match[1], ", ", "; ")
		}
		if match := publisherRegex.FindStringSubmatch(text); len(match) > 1 {
			result.Publisher = strings.ReplaceAll(match[1], ", ", "; ")
		}
		if match := albumTypeRegex.FindStringSubmatch(text); len(match) > 1 {
			result.AlbumType = strings.ReplaceAll(match[1], ", ", "; ")
			if result.AlbumType == "Gamerip" {
				result.AlbumType = "Soundtrack"
			}
		}
	})

	// Get links to images
	doc.Find("#pageContent .albumImage a").Each(func(i int, s *goquery.Selection) {
		imgUrl, ok := s.Attr("href")
		if ok {
			imgUrl, _ = joinUrl(albumUrl, imgUrl)
			result.ImageUrls = append(result.ImageUrls, imgUrl)
		}
	})

	// Get links to tracks
	doc.Find("#pageContent .playlistDownloadSong a").Each(func(i int, s *goquery.Selection) {
		pageUrl, ok := s.Attr("href")
		if ok {
			pageUrl, _ = joinUrl(albumUrl, pageUrl)
			result.Tracks = append(result.Tracks, TrackInfo{PageUrl: pageUrl})
		}
	})

	return &result, nil
}

func FetchTrackDownloadUrl(ctx context.Context, httpClient HttpDoClient, pageUrl string) (string, error) {
	body, err := getUrl(ctx, httpClient, pageUrl)
	if err != nil {
		return "", fmt.Errorf("failed to fetch track page: %w", err)
	}
	defer body.Close()
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse html file for track page: %w", err)
	}

	result := ""
	selectors := []string{
		"#pageContent a span:contains('Click here to download as FLAC')",
		"#pageContent a span:contains('Click here to download as MP3')",
	}
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			downloadUrl, ok := s.Parent().Attr("href")
			if ok {
				downloadUrl, _ = joinUrl(pageUrl, downloadUrl)
				result = downloadUrl
			}
		})
		if result != "" {
			break
		}
	}
	if result == "" {
		return "", fmt.Errorf("failed to find download link")
	}

	return result, nil
}

type IOer interface {
	MkdirAll(string, os.FileMode) error
	Create(string) (io.WriteCloser, error)
	Stat(string) (os.FileInfo, error)
}

func FetchAlbum(ctx context.Context, httpClient HttpDoClient, ioer IOer, logger *slog.Logger, workPath, albumUrl string, noDownload bool) (*AlbumInfo, error) {
	logger.Info("fetching from " + albumUrl)
	albumInfo, err := FetchAlbumInfo(ctx, httpClient, albumUrl)
	if err != nil {
		return nil, err
	}
	logger.Info(
		"fetched info",
		"name", albumInfo.Name,
		"year", albumInfo.Year,
		"developer", albumInfo.Developer,
		"publisher", albumInfo.Publisher,
		"albumType", albumInfo.AlbumType,
		"images", len(albumInfo.ImageUrls),
		"tracks", len(albumInfo.Tracks),
	)

	folderName := path.Join(workPath, sanitizeFilename(albumInfo.Name))

	err = ioer.MkdirAll(folderName, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	download := func(u, kind string) {
		if noDownload {
			return
		}
		logger.Info("downloading from " + u)
		unescaped, _ := URL.QueryUnescape(u)
		fileName := path.Join(folderName, sanitizeFilename(path.Base(unescaped)))
		// Skip if exists
		if _, err := ioer.Stat(fileName); err == nil {
			logger.Info("skipped " + fileName)
			return
		}
		body, err := getUrl(ctx, httpClient, unescaped)
		if err != nil {
			slog.Error("failed to download " + kind + ": " + err.Error())
			return
		}
		defer body.Close()
		file, err := ioer.Create(fileName)
		if err != nil {
			slog.Error("failed to create " + kind + " file: " + err.Error())
			return
		}
		defer file.Close()
		_, err = io.Copy(file, body)
		if err != nil {
			slog.Error("failed to write " + kind + " file: " + err.Error())
			return
		}
	}

	for _, imgUrl := range albumInfo.ImageUrls {
		download(imgUrl, "image")
	}
	if len(albumInfo.ImageUrls) == 0 {
		logger.Info("no images found")
	}

	for i := range albumInfo.Tracks {
		t := &albumInfo.Tracks[i]
		trackUrl, err := FetchTrackDownloadUrl(ctx, httpClient, t.PageUrl)
		if err != nil {
			slog.Error("failed to fetch track download url: " + err.Error())
			continue
		}
		download(trackUrl, "track")
		t.SongUrl = trackUrl
	}
	if len(albumInfo.Tracks) == 0 {
		logger.Info("no tracks found")
	}

	// Write summary
	logger.Info("writing summary")
	if summaryFile, err := ioer.Create(path.Join(folderName, "info.json")); err != nil {
		slog.Error("failed to create summary file: " + err.Error())
	} else {
		defer summaryFile.Close()
		encoder := json.NewEncoder(summaryFile)
		encoder.SetIndent("", "  ")
		encoder.Encode(albumInfo)
	}

	// Write a Windows shortcut file
	logger.Info("writing shortcut file")
	if lnkFile, err := ioer.Create(path.Join(folderName, "page.url")); err != nil {
		slog.Error("failed to create lnk file: " + err.Error())
	} else {
		defer lnkFile.Close()
		lnkFile.Write([]byte("[{000214A0-0000-0000-C000-000000000046}]\r\n"))
		lnkFile.Write([]byte("Prop3=19,11\r\n"))
		lnkFile.Write([]byte("[InternetShortcut]\r\n"))
		lnkFile.Write([]byte("IDList=\r\n"))
		lnkFile.Write([]byte("URL=" + albumUrl + "\r\n"))
	}

	return albumInfo, nil
}
