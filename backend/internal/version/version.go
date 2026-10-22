// Package version identifies the running build.
//
// An image is built once per commit and then promoted to releases by adding
// registry tags, so the build only knows its commit. Image builds set it with
//
//	go build -ldflags "-X family-cookbook/backend/internal/version.commit=abc1234"
//
// The release version is supplied when deploying, via APP_VERSION.
package version

import (
	"os"
	"runtime/debug"
)

// commit is set at build time; see the package comment.
var commit string

// Commit returns the commit this binary was built from: the value set at
// build time, else what `go build` recorded from the checkout (with "-dirty"
// for uncommitted changes), else "dev".
func Commit() string {
	if commit != "" {
		return commit
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	var rev string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "dev"
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if dirty {
		rev += "-dirty"
	}
	return rev
}

// Release returns the release version given at deploy time (APP_VERSION),
// or "" when none was given.
func Release() string {
	return os.Getenv("APP_VERSION")
}
