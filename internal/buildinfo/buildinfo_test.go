package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestRead(t *testing.T) {
	vcs := func(revision, modified string) *debug.BuildInfo {
		return &debug.BuildInfo{
			// what Go 1.24+ stamps on a build from a checkout; never a release
			Main: debug.Module{Version: "v1.2.1-0.20260101000000-c8249e1a2b3c+dirty"},
			Settings: []debug.BuildSetting{
				{Key: "vcs", Value: "git"},
				{Key: "vcs.revision", Value: revision},
				{Key: "vcs.modified", Value: modified},
			},
		}
	}
	const sha = "c8249e1a2b3c4d5e6f708192a3b4c5d6e7f80912"

	tests := []struct {
		name           string
		releaseVersion string
		bi             *debug.BuildInfo
		isRelease      bool
		version        string
		commit         string
		modified       bool
		str            string
	}{
		{
			name:           "release",
			releaseVersion: "v0.1.0",
			bi:             vcs(sha, "false"),
			isRelease:      true,
			version:        "v0.1.0",
			commit:         "c8249e1",
			str:            "v0.1.0",
		},
		{
			name:           "prerelease",
			releaseVersion: "v0.2.0-rc.1",
			bi:             vcs(sha, "false"),
			isRelease:      true,
			version:        "v0.2.0-rc.1",
			commit:         "c8249e1",
			str:            "v0.2.0-rc.1",
		},
		{
			name:           "invalid release version",
			releaseVersion: "0.1.0",
			bi:             vcs(sha, "false"),
			version:        "dev",
			commit:         "c8249e1",
			str:            "dev (c8249e1)",
		},
		{
			name:    "dev with commit",
			bi:      vcs(sha, "false"),
			version: "dev",
			commit:  "c8249e1",
			str:     "dev (c8249e1)",
		},
		{
			name:     "dev modified",
			bi:       vcs(sha, "true"),
			version:  "dev",
			commit:   "c8249e1",
			modified: true,
			str:      "dev (c8249e1, modified)",
		},
		{
			name:    "no vcs info",
			bi:      &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			version: "dev",
			str:     "dev",
		},
		{
			name:    "no build info",
			version: "dev",
			str:     "dev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := read(tt.releaseVersion, tt.bi)
			if got := i.IsRelease(); got != tt.isRelease {
				t.Errorf("IsRelease() = %v, want %v", got, tt.isRelease)
			}
			if got := i.Version(); got != tt.version {
				t.Errorf("Version() = %q, want %q", got, tt.version)
			}
			if got := i.Commit(); got != tt.commit {
				t.Errorf("Commit() = %q, want %q", got, tt.commit)
			}
			if got := i.Modified(); got != tt.modified {
				t.Errorf("Modified() = %v, want %v", got, tt.modified)
			}
			if got := i.String(); got != tt.str {
				t.Errorf("String() = %q, want %q", got, tt.str)
			}
		})
	}
}
