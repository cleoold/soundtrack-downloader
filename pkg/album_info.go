package pkg

type AlbumInfo struct {
	Url       string
	Name      string
	Platforms string
	Year      string
	Developer string
	Publisher string
	AlbumType string

	ImageUrls []string
	Tracks    []TrackInfo
}

type TrackInfo struct {
	PageUrl string
	SongUrl string
}
