package runtime

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const (
	// hostRequiredDarwinArch is the only macOS architecture Apple's container
	// runtime supports; Intel Macs cannot run it.
	hostRequiredDarwinArch = "arm64"
	// minHostMacOSMajor is the oldest macOS release Apple's container runtime
	// supports (https://github.com/apple/container#requirements).
	minHostMacOSMajor = 26
)

// checkDarwinHost validates the macOS host for the Apple container runtime.
// goarch is the architecture this binary was built for and swVersOutput is the
// output of `sw_vers -productVersion`.
func checkDarwinHost(goarch, swVersOutput string) error {
	if goarch != hostRequiredDarwinArch {
		return fmt.Errorf("ned requires a Mac with Apple silicon and a darwin/%s build; this binary is darwin/%s. Apple's container runtime does not support Intel Macs or Rosetta-translated clients", hostRequiredDarwinArch, goarch)
	}
	version, ok := parseMacOSVersion(swVersOutput)
	if !ok || !version.Valid() {
		return fmt.Errorf("ned requires macOS %d or later; unable to parse macOS version from %q", minHostMacOSMajor, strings.TrimSpace(swVersOutput))
	}
	if version.Major < minHostMacOSMajor {
		return fmt.Errorf("ned requires macOS %d or later for Apple's container runtime; detected macOS %s. Upgrade macOS to continue", minHostMacOSMajor, version.String())
	}
	return nil
}

// MacOSVersion is a parsed `sw_vers -productVersion` value.
type MacOSVersion struct {
	Major int
	Minor int
	Patch int
	Raw   string
}

func (v MacOSVersion) Valid() bool {
	return v.Major > 0
}

func (v MacOSVersion) String() string {
	if strings.TrimSpace(v.Raw) != "" {
		return strings.TrimSpace(v.Raw)
	}
	if v.Major > 0 {
		if v.Patch > 0 {
			return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
		}
		if v.Minor > 0 {
			return fmt.Sprintf("%d.%d", v.Major, v.Minor)
		}
		return fmt.Sprintf("%d", v.Major)
	}
	return "unknown"
}

func parseMacOSVersion(output string) (MacOSVersion, bool) {
	raw := strings.TrimSpace(output)
	if raw == "" {
		return MacOSVersion{}, false
	}
	parts := strings.Split(raw, ".")
	version := MacOSVersion{Raw: raw}
	parsePart := func(idx int) (int, bool) {
		if idx >= len(parts) {
			return 0, true
		}
		value := strings.TrimSpace(parts[idx])
		if value == "" {
			return 0, true
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	major, ok := parsePart(0)
	if !ok || major == 0 {
		return MacOSVersion{}, false
	}
	minor, ok := parsePart(1)
	if !ok {
		return MacOSVersion{}, false
	}
	patch, ok := parsePart(2)
	if !ok {
		return MacOSVersion{}, false
	}
	version.Major = major
	version.Minor = minor
	version.Patch = patch
	return version, true
}

func commandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s failed: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
