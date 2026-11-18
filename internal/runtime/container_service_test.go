package runtime

import "testing"

func TestParseContainerRuntimeServiceRunningCurrentStatus(t *testing.T) {
	output := []byte(`{
  "status": "running",
  "client": {"version": "1.6.0"},
  "server": {"version": "1.6.0"},
  "paths": {
    "appRoot": "/Users/jf/Library/Application Support/com.apple.container",
    "installRoot": "/opt/homebrew/Cellar/container/1.6.0"
  },
  "containers": 0,
  "images": 3
}`)
	running, err := parseContainerRuntimeServiceRunning(output)
	if err != nil {
		t.Fatalf("parseContainerRuntimeServiceRunning failed: %v", err)
	}
	if !running {
		t.Fatal("expected running status")
	}
}

func TestParseContainerRuntimeServiceRunningStopped(t *testing.T) {
	running, err := parseContainerRuntimeServiceRunning([]byte(`{"status":"stopped","paths":{"appRoot":"/tmp/container"}}`))
	if err != nil {
		t.Fatalf("parseContainerRuntimeServiceRunning failed: %v", err)
	}
	if running {
		t.Fatal("expected stopped status")
	}
}

func TestParseContainerRuntimeServiceRunningRejectsMissingStatus(t *testing.T) {
	if _, err := parseContainerRuntimeServiceRunning([]byte(`{"paths":{"appRoot":"/tmp/container"}}`)); err == nil {
		t.Fatal("expected missing status to be rejected")
	}
}

func TestParseContainerRuntimeServiceRunningRejectsLegacyStatusText(t *testing.T) {
	if _, err := parseContainerRuntimeServiceRunning([]byte("status running\n")); err == nil {
		t.Fatal("expected legacy status text to be rejected")
	}
}
