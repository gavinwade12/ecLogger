// Package buildinfo reports which version of ecLogger is running.
//
// Release builds have their version set at link time:
//
//	go build -ldflags "-X github.com/gavinwade12/ecLogger/internal/buildinfo.releaseVersion=v0.1.0"
//
// Anything else is a dev build, described by the commit it was built from.
package buildinfo

import (
	"runtime/debug"

	"golang.org/x/mod/semver"
)

// releaseVersion is set by CI for release builds.
var releaseVersion string

// shortCommitLen is how much of the commit hash Commit returns.
const shortCommitLen = 7

// Info describes a build.
type Info struct {
	version  string
	commit   string
	modified bool
}

// current is this build's Info.
var current = read(releaseVersion, readBuildInfo())

func readBuildInfo() *debug.BuildInfo {
	bi, _ := debug.ReadBuildInfo()
	return bi
}

// read returns the Info for a build with the given release version and build
// info, either of which may be missing.
//
// Only a valid semver releaseVersion makes a release. The module version in
// bi isn't used: since Go 1.24 it's a pseudo-version for any build from a
// checkout, which would make every local build look like a release.
func read(releaseVersion string, bi *debug.BuildInfo) Info {
	var info Info
	if semver.IsValid(releaseVersion) {
		info.version = releaseVersion
	}
	if bi == nil {
		return info
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			info.commit = s.Value
			if len(info.commit) > shortCommitLen {
				info.commit = info.commit[:shortCommitLen]
			}
		case "vcs.modified":
			info.modified = s.Value == "true"
		}
	}
	return info
}

// IsRelease returns whether this is a release build.
func (i Info) IsRelease() bool { return i.version != "" }

// Version returns the release version, such as v0.1.0, or "dev".
func (i Info) Version() string {
	if i.IsRelease() {
		return i.version
	}
	return "dev"
}

// Commit returns the short hash of the commit built from, or "" if unknown.
func (i Info) Commit() string { return i.commit }

// Modified returns whether the build had uncommitted changes.
func (i Info) Modified() bool { return i.modified }

// String describes the build for display: v0.1.0 for a release, otherwise
// dev, dev (c8249e1) or dev (c8249e1, modified).
func (i Info) String() string {
	if i.IsRelease() || i.commit == "" {
		return i.Version()
	}
	if i.modified {
		return "dev (" + i.commit + ", modified)"
	}
	return "dev (" + i.commit + ")"
}

// IsRelease returns whether this is a release build.
func IsRelease() bool { return current.IsRelease() }

// Version returns this build's release version, such as v0.1.0, or "dev".
func Version() string { return current.Version() }

// Commit returns the short hash of the commit this was built from, or "" if
// unknown.
func Commit() string { return current.Commit() }

// Modified returns whether this was built with uncommitted changes.
func Modified() bool { return current.Modified() }

// String describes this build for display.
func String() string { return current.String() }
