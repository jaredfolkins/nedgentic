package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	goruntime "runtime"
	"strings"
	"sync"
)

type containerSystemStatusPayload struct {
	Status string                      `json:"status"`
	Paths  *containerSystemStatusPaths `json:"paths,omitempty"`
}

type containerSystemStatusPaths struct {
	AppRoot string `json:"appRoot"`
}

var (
	containerServiceStatus = ContainerSystemStatus
	containerServiceStart  = StartContainerRuntime

	containerServiceCacheMu sync.Mutex
	containerServiceReady   = map[string]bool{}
)

func ContainerRuntimeServiceRunning(bin string) (bool, error) {
	if !containerServiceSupported() {
		return false, nil
	}
	key, err := normalizeContainerServiceBin(bin)
	if err != nil {
		return false, err
	}
	if containerServiceCachedReady(key) {
		return true, nil
	}
	out, err := containerServiceStatus(key, "json")
	if err != nil {
		return false, err
	}
	running, err := parseContainerRuntimeServiceRunning(out)
	if err != nil {
		return false, err
	}
	if running {
		rememberContainerServiceReady(key)
	}
	return running, nil
}

func StartContainerRuntimeService(bin string) error {
	if !containerServiceSupported() {
		return nil
	}
	key, err := normalizeContainerServiceBin(bin)
	if err != nil {
		return err
	}
	if containerServiceCachedReady(key) {
		return nil
	}
	if err := containerServiceStart(io.Discard, key, RuntimeActionOptions{}); err != nil {
		return err
	}
	rememberContainerServiceReady(key)
	return nil
}

func EnsureContainerRuntimeStarted(bin string) (bool, error) {
	running, statusErr := ContainerRuntimeServiceRunning(bin)
	if statusErr == nil && running {
		return false, nil
	}
	if err := StartContainerRuntimeService(bin); err != nil {
		if statusErr != nil {
			return false, fmt.Errorf("container service status check failed: %v; start failed: %w", statusErr, err)
		}
		return false, err
	}
	return true, nil
}

func parseContainerRuntimeServiceRunning(out []byte) (bool, error) {
	var payload containerSystemStatusPayload
	if err := json.Unmarshal(sanitizeJSONOutput(out), &payload); err != nil {
		return false, fmt.Errorf("failed to parse container system status: %v", err)
	}
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status == "" {
		return false, fmt.Errorf("container system status did not include a status value")
	}
	return strings.HasPrefix(status, "running"), nil
}

func normalizeContainerServiceBin(bin string) (string, error) {
	trimmed := strings.TrimSpace(bin)
	if trimmed == "" {
		return "", fmt.Errorf("container runtime path is required")
	}
	return trimmed, nil
}

func containerServiceSupported() bool {
	return goruntime.GOOS == "darwin"
}

func containerServiceCachedReady(bin string) bool {
	containerServiceCacheMu.Lock()
	defer containerServiceCacheMu.Unlock()
	return containerServiceReady[bin]
}

func rememberContainerServiceReady(bin string) {
	containerServiceCacheMu.Lock()
	defer containerServiceCacheMu.Unlock()
	containerServiceReady[bin] = true
}
