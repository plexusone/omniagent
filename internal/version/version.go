// Package version provides build and version information for envoy.
package version

import (
	"fmt"
	"runtime"
)

// Build information, set via ldflags.
var (
	Version   = "0.1.0"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Info contains version information.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// Get returns the current version information.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string.
func (i Info) String() string {
	return fmt.Sprintf("envoy %s (%s) built %s with %s for %s",
		i.Version, i.Commit, i.BuildDate, i.GoVersion, i.Platform)
}
