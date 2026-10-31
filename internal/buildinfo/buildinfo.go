// Package buildinfo holds the build version and the User-Agent text.
package buildinfo

import (
	"fmt"
	"os"
	"runtime"
)

// Version is the release version. The build sets it with -ldflags -X.
var Version = "dev"

// Commit and Date identify the build. The build sets them with -ldflags -X.
var Commit, Date string

// UserAgent returns the User-Agent header value for HTTP requests.
func UserAgent() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return fmt.Sprintf("sermo-cli/%s (%s/%s; %s)", Version, runtime.GOOS, runtime.GOARCH, host)
}
