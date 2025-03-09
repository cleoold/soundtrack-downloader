package pkg

type AlbumInfo struct {
	Url           string
	Name          string
	Platforms     string
	Year          string
	Developer     string
	Publisher     string
	CatalogNumber string
	AlbumType     string

	ImageUrls []string
	Tracks    []TrackInfo
}

type TrackInfo struct {
	Name        string
	DiscNumber  string `json:",omitzero"`
	TrackNumber string
	PageUrl     string
	SongUrl     string
}
