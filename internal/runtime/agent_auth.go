package runtime

import (
	"fmt"
	"path/filepath"
	"strings"
)

// hostAgentAuthPaths are host locations where agent CLIs keep OAuth logins.
// Their refresh tokens rotate on use, so a copy in a VM that refreshes
// invalidates the host login (and the reverse). VMs sign in on their own
// disk with ned-agent-login instead.
var hostAgentAuthPaths = []string{
	".claude",
	".claude.json",
	".claude.json.backup",
	".codex",
	".gemini",
	".grok",
	".local/share/opencode",
}

// isHostAgentAuthPath reports whether a profile import host path is, or is
// inside, an agent CLI login location under the host home.
func isHostAgentAuthPath(hostPath string) bool {
	path := filepath.ToSlash(strings.TrimSpace(hostPath))
	if path == "" {
		return false
	}
	path = strings.TrimPrefix(path, "~/")
	for _, entry := range hostAgentAuthPaths {
		if path == entry || strings.HasPrefix(path, entry+"/") ||
			strings.HasSuffix(path, "/"+entry) || strings.Contains(path, "/"+entry+"/") {
			return true
		}
	}
	return false
}

// rejectHostAgentAuthImports fails fast when a profile copies or mounts host
// agent logins into the VM.
func rejectHostAgentAuthImports(prof *Profile) error {
	if prof == nil {
		return nil
	}
	var paths []string
	for _, mount := range prof.Files {
		if isHostAgentAuthPath(mount.HostPath) {
			paths = append(paths, mount.HostPath)
		}
	}
	for _, mount := range prof.Directories {
		if isHostAgentAuthPath(mount.HostPath) {
			paths = append(paths, mount.HostPath)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	return fmt.Errorf("imports host agent logins (%s); these tokens rotate, so a VM copy breaks the host login. Remove them from imports and sign in inside the VM with ned-agent-login", strings.Join(paths, ", "))
}
