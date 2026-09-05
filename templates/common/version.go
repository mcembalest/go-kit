package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// GoReleaser sets these; go install supplies its module version instead.
var version, commit string

func buildVersion(name string) string {
	v, revision, dirty := version, commit, "unknown"
	if info, ok := debug.ReadBuildInfo(); ok {
		if v == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if revision == "" {
					revision = setting.Value
				}
			case "vcs.modified":
				dirty = setting.Value
			}
		}
	}
	if v == "" {
		v = "dev"
	}
	if revision == "" {
		revision = "unknown"
	}
	return fmt.Sprintf("%s %s (commit=%s dirty=%s %s %s/%s)", name, v, revision, dirty, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
