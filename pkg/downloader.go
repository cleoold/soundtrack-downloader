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
	catalogRegex   = regexp.MustCompile(`(?m)Catalog Number:\s*(.+?)\s*$`)
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
		if match := catalogRegex.FindStringSubmatch(text); len(match) > 1 {
			result.CatalogNumber = match[1]
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

	CDIndex := doc.Find("#pageContent #songlist_header th:contains('CD')").Index()
	TrackIndex := doc.Find("#pageContent #songlist_header th:contains('#')").Index()
	NameIndex := doc.Find("#pageContent #songlist_header th:contains('Name')").Index()

	// Get links to tracks
	doc.Find("#pageContent #songlist tr:not(#songlist_header):not(#songlist_footer)").Each(func(i int, s *goquery.Selection) {
		trackInfo := TrackInfo{}
		s.Find("td").Each(func(j int, s *goquery.Selection) {
			switch j {
			case CDIndex:
				trackInfo.DiscNumber = strings.Trim(strings.TrimSpace(s.Text()), ".")
			case TrackIndex:
				trackInfo.TrackNumber = strings.Trim(strings.TrimSpace(s.Text()), ".")
			case NameIndex:
				trackInfo.Name = strings.TrimSpace(s.Text())
			}
		})
		s.Find("td a").Each(func(j int, s *goquery.Selection) {
			pageUrl, ok := s.Attr("href")
			if ok {
				pageUrl, _ = joinUrl(albumUrl, pageUrl)
				trackInfo.PageUrl = pageUrl
			}
		})
		result.Tracks = append(result.Tracks, trackInfo)
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

func fetchAlbum(
	ctx context.Context,
	httpClient HttpDoClient,
	logger *slog.Logger,
	osMkdirAll func(string, os.FileMode) error,
	osCreate func(string) (io.WriteCloser, error),
	osStat func(string) (os.FileInfo, error),
	workPath,
	albumUrl string,
	noDownloadImage,
	noDownloadTrack,
	overwrite bool,
	trackNumberSet TrackNumberSet,
) (*AlbumInfo, string, error) {
	logger.Info("fetching from " + albumUrl)
	albumInfo, err := FetchAlbumInfo(ctx, httpClient, albumUrl)
	if err != nil {
		return nil, "", err
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

	err = osMkdirAll(folderName, os.ModePerm)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create directory: %w", err)
	}

	download := func(u, kind string) {
		logger.Info("downloading from " + u)
		unescaped, _ := URL.QueryUnescape(u)
		fileName := path.Join(folderName, sanitizeFilename(path.Base(unescaped)))
		if !overwrite {
			if _, err := osStat(fileName); err == nil {
				logger.Info("skipped " + fileName)
				return
			}
		}
		body, err := getUrl(ctx, httpClient, unescaped)
		if err != nil {
			slog.Error("failed to download " + kind + ": " + err.Error())
			return
		}
		defer body.Close()
		file, err := osCreate(fileName)
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

	if !noDownloadImage {
		for _, imgUrl := range albumInfo.ImageUrls {
			download(imgUrl, "image")
		}
		if len(albumInfo.ImageUrls) == 0 {
			logger.Info("no images found")
		}
	}

	if !noDownloadTrack {
		for i := range albumInfo.Tracks {
			t := &albumInfo.Tracks[i]
			if !trackNumberSet.Contains(t) {
				slog.Debug("skipping track " + t.Name)
				continue
			}
			trackUrl, err := FetchTrackDownloadUrl(ctx, httpClient, t.PageUrl)
			if err != nil {
				slog.Error("failed to fetch track download url: " + err.Error())
				continue
			}
			t.SongUrl = trackUrl
			download(trackUrl, "track")
		}
		if len(albumInfo.Tracks) == 0 {
			logger.Info("no tracks found")
		}
	}

	// Write summary
	logger.Info("writing summary")
	if summaryFile, err := osCreate(path.Join(folderName, "info.json")); err != nil {
		slog.Error("failed to create summary file: " + err.Error())
	} else {
		defer summaryFile.Close()
		encoder := json.NewEncoder(summaryFile)
		encoder.SetIndent("", "  ")
		encoder.Encode(albumInfo)
	}

	// Write a Windows shortcut file
	logger.Info("writing shortcut file")
	if lnkFile, err := osCreate(path.Join(folderName, "page.url")); err != nil {
		slog.Error("failed to create lnk file: " + err.Error())
	} else {
		defer lnkFile.Close()
		lnkFile.Write([]byte("[{000214A0-0000-0000-C000-000000000046}]\r\n"))
		lnkFile.Write([]byte("Prop3=19,11\r\n"))
		lnkFile.Write([]byte("[InternetShortcut]\r\n"))
		lnkFile.Write([]byte("IDList=\r\n"))
		lnkFile.Write([]byte("URL=" + albumUrl + "\r\n"))
	}

	return albumInfo, folderName, nil
}

func FetchAlbum(
	ctx context.Context,
	httpClient HttpDoClient,
	logger *slog.Logger,
	workPath,
	albumUrl string,
	noDownloadImage,
	noDownloadTrack,
	overwrite bool,
	trackNumberSet TrackNumberSet,
) (*AlbumInfo, string, error) {
	osCreate := func(name string) (io.WriteCloser, error) {
		return os.Create(name) // covariance
	}
	return fetchAlbum(ctx, httpClient, logger, os.MkdirAll, osCreate, os.Stat, workPath, albumUrl, noDownloadImage, noDownloadTrack, overwrite, trackNumberSet)
}

type TrackNumberKey struct{ DiscNumber, TrackNumber string }
type TrackNumberSet map[TrackNumberKey]struct{}

// A nil set contains everything
func (s TrackNumberSet) Contains(info *TrackInfo) bool {
	if s == nil {
		return true
	}
	disc := strings.TrimLeft(info.DiscNumber, "0")
	track := strings.TrimLeft(info.TrackNumber, "0")
	_, ok := s[TrackNumberKey{disc, track}]
	return ok
}

func (s TrackNumberSet) Add(key TrackNumberKey) {
	disc := strings.TrimLeft(key.DiscNumber, "0")
	track := strings.TrimLeft(key.TrackNumber, "0")
	s[TrackNumberKey{disc, track}] = struct{}{}
}
