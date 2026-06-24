package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func init() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.revision":
					if len(s.Value) >= 7 {
						Commit = s.Value[:7]
					} else {
						Commit = s.Value
					}
				case "vcs.time":
					BuildDate = s.Value
				}
			}
		}
	}
}

func String() string {
	return fmt.Sprintf("ztutor %s (%s %s/%s) commit=%s built=%s",
		Version, runtime.Version(), runtime.GOOS, runtime.GOARCH, Commit, BuildDate)
}
