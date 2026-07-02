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

// Tag returns the version formatted as a GitHub release tag: "v<Version>".
// When Version is "dev", it returns "dev".
func Tag() string {
	if Version == "dev" {
		return "dev"
	}
	return "v" + Version
}

// IsDev reports whether this is a development build (not a tagged release).
func IsDev() bool {
	return Version == "dev"
}
