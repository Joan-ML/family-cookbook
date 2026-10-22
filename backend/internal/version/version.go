// Package version holds the application version.
//
// Release builds set it from the git tag (see the Dockerfile):
//
//	go build -ldflags "-X family-cookbook/backend/internal/version.Version=v1.2.3"
package version

// Version is the application version, or "dev" for unversioned builds.
var Version = "dev"
