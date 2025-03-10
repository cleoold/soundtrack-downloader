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

func (t tagFlags) String() string {
	return fmt.Sprintf("%+v", map[string]string(t))
}

func (t tagFlags) Set(value string) error {
	res := strings.SplitN(value, "=", 2)
	if len(res) != 2 {
		return fmt.Errorf("invalid tag format: %s", value)
	}
	t[strings.ToUpper(res[0])] = res[1]
	return nil
}

type overrideFlags pkg.TagKeySet

func (o overrideFlags) String() string {
	return fmt.Sprintf("%+v", pkg.TagKeySet(o))
}

func (o overrideFlags) Set(value string) error {
	for part := range strings.SplitSeq(value, ",") {
		pkg.TagKeySet(o).Add(strings.TrimSpace(part))
	}
	return nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flag.Usage = cmd.PrintUsage
	folderFlag := flag.String("folder", "", "Folder to fix tags")
	tags := tagFlags{}
	flag.Var(&tags, "tag", "Tag to set. Format: -tag key=value. Multiple are supported. Available keys include 'ALBUM', 'DATE', 'ALBUMARTIST', 'ARTIST', 'GENRE' and so on. See https://taglib.org/api/p_propertymapping.html for more. If provided, this option has higher precedence than ones scanned by -read-album-info.")
	inferNamesFlag := flag.Bool("infer-names", false, "Infer track names from file names. Default: false")
	overwrites := overrideFlags{}
	flag.Var(&overwrites, "overwrite", "Overwrite existing tags by their key names (example: -overwrite ALBUM,ARTIST,TRACKNUMBER). Special value '*' means to overwrite all tags. Default: none")
	readAlbumInfoFlag := flag.Bool("read-album-info", false, "Read album info from info.json. Default: false")
	noFixFlag := flag.Bool("no-fix", false, "Only print the proposed changes but don't fix tags. Default: false")
	flag.Parse()
	if *folderFlag == "" {
		flag.Usage()
		logger.Error("folder is required")
		return
	}

	err := pkg.FixTags(logger, tags, nil, pkg.TagKeySet(overwrites), *folderFlag, *inferNamesFlag, *readAlbumInfoFlag, *noFixFlag)
	if err != nil {
		logger.Error(err.Error())
	}
}
