package runtime

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestIsMissingContainerConfigDeleteError(t *testing.T) {
	output := "failed to delete container openclaw: Error Domain=NSCocoaErrorDomain Code=260 UserInfo={NSFilePath=/Users/jf/Library/Application Support/com.apple.container/containers/openclaw/config.json, NSUnderlyingError=0 {Error Domain=NSPOSIXErrorDomain Code=2 \"No such file or directory\"}}"
	if !isMissingContainerConfigDeleteError(output) {
		t.Fatalf("expected missing config delete error to be detected")
	}
	if isMissingContainerConfigDeleteError("config.json: permission denied") {
		t.Fatalf("expected permission denied to be ignored")
	}
}

func TestParseContainerConfigPath(t *testing.T) {
	output := "NSFilePath=/Users/jf/Library/Application Support/com.apple.container/containers/openclaw/config.json, NSURL=file:///Users/jf/Library/Application%20Support/com.apple.container/containers/openclaw/config.json"
	path, ok := parseContainerConfigPath(output)
	if !ok {
		t.Fatalf("expected config path to parse")
	}
	expected := "/Users/jf/Library/Application Support/com.apple.container/containers/openclaw/config.json"
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}

func TestContainerStateDirFromOutput(t *testing.T) {
	home := "/Users/jf"
	output := "NSFilePath=/Users/jf/Library/Application Support/com.apple.container/containers/openclaw/config.json, NSURL=file:///Users/jf/Library/Application%20Support/com.apple.container/containers/openclaw/config.json"
	dir, ok := containerStateDirFromOutput(output, "openclaw", home)
	if !ok {
		t.Fatalf("expected container state dir to resolve")
	}
	expected := filepath.Join(home, "Library", "Application Support", "com.apple.container", "containers", "openclaw")
	if dir != expected {
		t.Fatalf("expected %q, got %q", expected, dir)
	}
}

func TestContainerStateDirFromOutputNameMismatch(t *testing.T) {
	home := "/Users/jf"
	output := "NSFilePath=/Users/jf/Library/Application Support/com.apple.container/containers/openclaw/config.json"
	if _, ok := containerStateDirFromOutput(output, "other", home); ok {
		t.Fatalf("expected name mismatch to be rejected")
	}
}

func TestContainerStateDirFromNameGuards(t *testing.T) {
	home := "/Users/jf"
	if _, ok := containerStateDirFromName("../bad", home); ok {
		t.Fatalf("expected traversal name to be rejected")
	}
	if _, ok := containerStateDirFromName("bad/name", home); ok {
		t.Fatalf("expected path separator to be rejected")
	}
}

func TestExtractContainerConfiguration(t *testing.T) {
	output := []byte("log line\n[{\"status\":\"stopped\",\"configuration\":{\"id\":\"openclaw\",\"labels\":{\"nedgentic\":\"1\"}}}]")
	config, err := extractContainerConfiguration(output)
	if err != nil {
		t.Fatalf("expected configuration to parse: %v", err)
	}
	var parsed struct {
		ID     string            `json:"id"`
		Labels map[string]string `json:"labels"`
	}
	if err := json.Unmarshal(config, &parsed); err != nil {
		t.Fatalf("expected config json to unmarshal: %v", err)
	}
	if parsed.ID != "openclaw" {
		t.Fatalf("expected id openclaw, got %q", parsed.ID)
	}
	if parsed.Labels["nedgentic"] != "1" {
		t.Fatalf("expected label to be present")
	}
}
