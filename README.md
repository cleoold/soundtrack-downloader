# soundtrack-downloader

Script that downloads soundtracks from khinsider.com.

## Building

Requires Go installed, run:

```bash
make build
```

Executables will be in the `bin/` directory.

## Basic Usage

To download an album, first obtain its URL, then run:

```bash
bin/downloader -url https://downloads.khinsider.com/game-soundtracks/album/<some-album>
```

This will create a folder in the current directory with the album's name and download all the images and tracks in it. It also creates an `info.json` file describing the information of the album ([schema](./pkg/album_info.go)), as well as a Windows link file to the URL.

At times the downloaded tracks may not have all the metadata so won't display properly in some music players (e.g. Jellyfin). To download while also fixing the metadata using available information, `-fix-tags` can be used:

```bash
bin/downloader -url https://downloads.khinsider.com/game-soundtracks/album/<some-album> -fix-tags
```

It will then, if missing, populate metadata fields such as "Artist", "Album name" (obtained from the website), "Title" and "Track number" (obtained from the track's filename).

The metadata-fixing process can also be done retroactively after the download by using

```bash
bin/meta -folder <Some Folder> -read-album-info
```

here the metadata is obtained using the `info.json` file created during the download.

You can provide tags yourself that have higher precedence than the ones obtained from the `info.json` file:

```bash
bin/meta -folder <Some Folder> -read-album-info -tag ARTIST=SomeArtist -tag ALBUM=SomeAlbum
```

## Synopsis

```
Usage of bin/downloader:
  -fix-tags
        Fix tags of the downloaded files. Default: false
  -no-download
        Combine no-download-image and no-download-track. Default: false
  -no-download-image
        Don't download images. Default: false
  -no-download-track
        Don't download tracks. Default: false
  -overwrite
        Redownload existing files. This option does not affect generation of info.json and link. Default: false
  -track value
        Tracks to download. Format: [disc number-]track number. Example: 1-1,1-2. Default to all tracks.
  -url string
        URL to download

Usage of bin/meta:
  -folder string
        Folder to fix tags
  -infer-names
        Infer names from file names. Default: true (default true)
  -no-fix
        Only print the proposed changes but don't fix tags. Default: false
  -overwrite
        Overwrite existing tags. Default: false
  -read-album-info
        Read album info from info.json. Default: false
  -tag value
        Tag to set. Format: key=value. Multiple are supported. Available keys include 'ALBUM', 'DATE', 'ALBUMARTIST', 'ARTIST', 'GENRE' and so on. See https://taglib.org/api/p_propertymapping.html for more.
```
