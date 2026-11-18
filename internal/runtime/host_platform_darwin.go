package runtime

import (
	"fmt"
	stdruntime "runtime"
)

var hostPlatformSwVers = func() (string, error) {
	return commandOutput("sw_vers", "-productVersion")
}

// CheckHostPlatform fails fast when the host cannot run Apple's container
// runtime: an Intel Mac, a non-arm64 build, or macOS older than 26.
func CheckHostPlatform() error {
	out, err := hostPlatformSwVers()
	if err != nil {
		return fmt.Errorf("host platform check failed: %w", err)
	}
	return checkDarwinHost(stdruntime.GOARCH, out)
}
