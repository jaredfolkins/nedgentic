package runtime

import (
	"testing"
	"time"
)

func TestWorkspaceMountPath(t *testing.T) {
	got := workspaceMountPath("/Users/jf/src/jf/nedgentic")
	want := "/mnt/ned-workspaces/Users/jf/src/jf/nedgentic"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestWaitForContainerReadyReturnsAfterProbeSucceeds(t *testing.T) {
	attempts := 0
	err := waitForContainerReady("demo", 3*time.Second, time.Second, func() error {
		attempts++
		if attempts == 3 {
			return nil
		}
		return errContainerNotReady
	}, func(_ time.Duration) {})
	if err != nil {
		t.Fatalf("waitForContainerReady error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestWaitForContainerReadyTimesOutWithConfiguredDuration(t *testing.T) {
	attempts := 0
	err := waitForContainerReady("demo", 3*time.Second, time.Second, func() error {
		attempts++
		return errContainerNotReady
	}, func(_ time.Duration) {})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if got := err.Error(); got != "timeout waiting up to 3s for container demo to be ready" {
		t.Fatalf("unexpected error: %q", got)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestParseContainerListOutputContainerOneStatusObject(t *testing.T) {
	output := []byte(`[{"configuration":{"id":"ned-demo","labels":{"ned.profile":"default","nedgentic":"1"},"image":{"reference":"docker.io/library/alpine:latest"}},"status":{"networks":[{"ipv4Address":"192.168.65.2/24"}],"state":"running"}}]`)
	entries, err := parseContainerListOutput(output)
	if err != nil {
		t.Fatalf("parseContainerListOutput error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Status != "running" {
		t.Fatalf("expected running status, got %q", entry.Status)
	}
	if entry.Configuration.ID != "ned-demo" {
		t.Fatalf("expected container id ned-demo, got %q", entry.Configuration.ID)
	}
	if entry.Configuration.Labels[ContainerLabelKey] != "1" {
		t.Fatalf("expected Nedgentic label to parse")
	}
	if len(entry.Networks) != 1 || entry.Networks[0].IPv4Address != "192.168.65.2/24" {
		t.Fatalf("expected status network address, got %#v", entry.Networks)
	}
}

var errContainerNotReady = testingErr("not ready")

type testingErr string

func (err testingErr) Error() string {
	return string(err)
}
