package runtime

import (
	"reflect"
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

func TestContainerExportArgsDefaultsToStdout(t *testing.T) {
	args, err := containerExportArgs("sandbox-vm", "")
	if err != nil {
		t.Fatalf("containerExportArgs error: %v", err)
	}
	want := []string{"export", "sandbox-vm"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: got %#v want %#v", args, want)
	}
}

func TestContainerExportArgsIncludesOutputPath(t *testing.T) {
	args, err := containerExportArgs("sandbox-vm", "/tmp/sandbox.tar")
	if err != nil {
		t.Fatalf("containerExportArgs error: %v", err)
	}
	want := []string{"export", "--output", "/tmp/sandbox.tar", "sandbox-vm"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: got %#v want %#v", args, want)
	}
}

func TestContainerExportArgsRejectsEmptyID(t *testing.T) {
	if _, err := containerExportArgs("", "/tmp/sandbox.tar"); err == nil {
		t.Fatal("expected error for empty container ID")
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

func TestImageListEntriesFromContainerOnePayload(t *testing.T) {
	output := []byte(`[{"configuration":{"creationDate":"2026-04-16T20:01:59Z","descriptor":{"annotations":{"org.opencontainers.image.created":"2026-04-16T20:01:59Z"},"digest":"sha256:b9449b17a699a3339321d15ad9689043b3e7b9662f714028d8bba3e15dd1c159","size":375},"name":"docker.io/library/alpine:latest"},"id":"b9449b17a699a3339321d15ad9689043b3e7b9662f714028d8bba3e15dd1c159","variants":[{"size":7564990}]}]`)
	images, err := parseContainerRuntimeImages(output)
	if err != nil {
		t.Fatalf("parseContainerRuntimeImages error: %v", err)
	}
	entries := imageListEntriesFromRuntimeImages(images, map[string]int64{
		"docker.io/library/alpine:latest": 10485760,
	})
	if len(entries) != 1 {
		t.Fatalf("expected 1 image entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Repository != "docker.io/library/alpine" {
		t.Fatalf("expected repository docker.io/library/alpine, got %q", entry.Repository)
	}
	if entry.Tag != "latest" {
		t.Fatalf("expected tag latest, got %q", entry.Tag)
	}
	if entry.ID != "b9449b17a699" {
		t.Fatalf("expected short image id, got %q", entry.ID)
	}
	if entry.Size != "10.0 MiB" {
		t.Fatalf("expected inspected size, got %q", entry.Size)
	}
	if entry.Created == "" || entry.Created == "N/A" {
		t.Fatalf("expected created time to be formatted, got %q", entry.Created)
	}
}

var errContainerNotReady = testingErr("not ready")

type testingErr string

func (err testingErr) Error() string {
	return string(err)
}
