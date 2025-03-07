package main

import (
	"flag"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cleoold/soundtrack-downloader/cmd"
	"github.com/cleoold/soundtrack-downloader/pkg"
)

type tagFlags map[string]string

func (t *tagFlags) String() string {
	return fmt.Sprintf("%+v", *t)
}

func (t *tagFlags) Set(value string) error {
	res := strings.SplitN(value, "=", 2)
	if len(res) != 2 {
		return fmt.Errorf("invalid tag format: %s", value)
	}
	(*t)[res[0]] = res[1]
	return nil
}

func main() {
	flag.Usage = cmd.PrintUsage
	folderFlag := flag.String("folder", "", "Folder to fix tags")
	tags := tagFlags{}
	flag.Var(&tags, "tag", "Tag to set. Format: key=value. Multiple are supported. Available keys include 'ALBUM', 'DATE', 'ALBUMARTIST', 'ARTIST', 'GENRE' and so on. See https://taglib.org/api/p_propertymapping.html for more.")
	inferNamesFlag := flag.Bool("infer-names", true, "Infer names from file names. Default: true")
	overwriteFlag := flag.Bool("overwrite", false, "Overwrite existing tags. Default: false")
	readAlbumInfoFlag := flag.Bool("read-album-info", false, "Read album info from info.json. Default: false")
	flag.Parse()
	if *folderFlag == "" {
		flag.Usage()
		slog.Error("folder is required")
		return
	}

	err := pkg.FixTags(slog.Default(), tags, *folderFlag, *inferNamesFlag, *overwriteFlag, *readAlbumInfoFlag)
	if err != nil {
		slog.Error(err.Error())
	}
}
