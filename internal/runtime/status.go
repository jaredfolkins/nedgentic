package runtime

import (
	"fmt"
	"os"
	"os/exec"
)

var statusCommandOutput = runtimeCommandOutput

func containerVersion(path string) (string, error) {
	out, err := statusCommandOutput(path, "--version")
	if err != nil {
		return "", fmt.Errorf("version check failed: %v", err)
	}
	lines := SanitizeStructuredCommandLines(out)
	if len(lines) == 0 {
		return "", fmt.Errorf("version output was empty")
	}
	return lines[0], nil
}

func runtimeCommandOutput(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}
