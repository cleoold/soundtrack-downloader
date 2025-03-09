package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
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
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flag.Usage = cmd.PrintUsage
	folderFlag := flag.String("folder", "", "Folder to fix tags")
	tags := tagFlags{}
	flag.Var(&tags, "tag", "Tag to set. Format: key=value. Multiple are supported. Available keys include 'ALBUM', 'DATE', 'ALBUMARTIST', 'ARTIST', 'GENRE' and so on. See https://taglib.org/api/p_propertymapping.html for more.")
	inferNamesFlag := flag.Bool("infer-names", false, "Infer track names from file names. Default: false")
	overwriteFlag := flag.Bool("overwrite", false, "Overwrite existing tags. Default: false")
	readAlbumInfoFlag := flag.Bool("read-album-info", false, "Read album info from info.json. Default: false")
	noFixFlag := flag.Bool("no-fix", false, "Only print the proposed changes but don't fix tags. Default: false")
	flag.Parse()
	if *folderFlag == "" {
		flag.Usage()
		logger.Error("folder is required")
		return
	}

	err := pkg.FixTags(logger, tags, nil, *folderFlag, *inferNamesFlag, *overwriteFlag, *readAlbumInfoFlag, *noFixFlag)
	if err != nil {
		logger.Error(err.Error())
	}
}
