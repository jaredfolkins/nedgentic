package runtime

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var containerSemverPattern = regexp.MustCompile(`\b([0-9]+)\.([0-9]+)\.([0-9]+)\b`)
var minimumSupportedContainerVersion = containerSemver{Major: 1, Minor: 6, Patch: 0}

type containerSemver struct {
	Major int
	Minor int
	Patch int
}

func (v containerSemver) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v containerSemver) lessThan(other containerSemver) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

func parseContainerSemver(raw string) (containerSemver, bool) {
	matches := containerSemverPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(matches) != 4 {
		return containerSemver{}, false
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return containerSemver{}, false
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return containerSemver{}, false
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return containerSemver{}, false
	}
	return containerSemver{Major: major, Minor: minor, Patch: patch}, true
}

func containerUpgradeHint() string {
	return fmt.Sprintf("Upgrade with `ned runtime install` or install the latest signed package from %s.", ContainerReleasePageURL)
}

// RequireContainerRuntimeVersion fails when the runtime at bin is missing,
// unparseable, or older than the minimum Ned supports.
func RequireContainerRuntimeVersion(bin string) error {
	if strings.TrimSpace(bin) == "" {
		return fmt.Errorf("container runtime path is required")
	}
	versionOutput, err := containerVersion(bin)
	if err != nil {
		return fmt.Errorf("nedgentic requires container %s or newer; failed to detect version: %v. %s", minimumSupportedContainerVersion, err, containerUpgradeHint())
	}
	if err := checkContainerVersion(versionOutput); err != nil {
		return fmt.Errorf("nedgentic %v. %s", err, containerUpgradeHint())
	}
	return nil
}

// checkContainerVersion compares `container --version` output against the
// minimum supported runtime. Every version gate goes through here.
func checkContainerVersion(versionOutput string) error {
	detected, ok := parseContainerSemver(versionOutput)
	if !ok {
		return fmt.Errorf("requires container %s or newer; unable to parse version from %q", minimumSupportedContainerVersion, strings.TrimSpace(versionOutput))
	}
	if detected.lessThan(minimumSupportedContainerVersion) {
		return fmt.Errorf("requires container %s or newer; detected %s", minimumSupportedContainerVersion, detected)
	}
	return nil
}
