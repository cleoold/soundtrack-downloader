package cmd

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
)

func getVersion() (goVersion, projectVersion, projectTime string) {
	if info, ok := debug.ReadBuildInfo(); ok {
		goVersion = info.Main.Version
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				projectVersion = setting.Value
			} else if setting.Key == "vcs.time" {
				projectTime = setting.Value
			}
		}
	}
	return
}

func PrintUsage() {
	goVersion, projectVersion, projectTime := getVersion()
	fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s (rev %s, compiled at %s, with Go %s):\n", os.Args[0], projectVersion, projectTime, goVersion)
	flag.PrintDefaults()
}
