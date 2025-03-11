package pkg

type AlbumInfo struct {
	Url              string
	Name             string
	AlternativeNames []string
	Platforms        []string
	Year             []string
	Developer        []string
	Publisher        []string
	CatalogNumber    []string
	AlbumType        []string

	Images []ImageInfo
	Tracks []TrackInfo
}

type ImageInfo struct {
	ImageUrl string
	ThumbUrl string
}

type TrackInfo struct {
	Name        string
	DiscNumber  string `json:",omitzero"`
	TrackNumber string
	PageUrl     string
	SongUrl     map[string]string
}
