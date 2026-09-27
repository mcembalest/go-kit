// Package kit is the small runtime side of go-kit: build identity and embedded Python workers.
package kit

import "runtime/debug"

// version is stamped by `go-kit ship` with -ldflags -X.
var version string

// Version is the release tag for shipped binaries, the module version for `go install`
// builds (a tag or a pseudo-version for @main), or "dev" for local builds.
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
