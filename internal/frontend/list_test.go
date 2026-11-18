package frontend

import (
	"testing"

	"nedgentic/internal/backend/runtime"
)

func TestBuildListRowsFiltersAndSorts(t *testing.T) {
	running := runtime.ContainerListEntry{
		Status: "Running",
	}
	running.Configuration.ID = "ned-b"
	running.Configuration.Image.Reference = "docker.io/library/alpine:latest"
	running.Configuration.Labels = map[string]string{
		runtime.ContainerLabelKey: "1",
		runtime.LabelProfileKey:   "b",
		runtime.LabelHostnameKey:  "host-b",
		runtime.LabelWorkspaceKey: "/work/b",
		runtime.LabelUserKey:      "bob",
	}

	stopped := runtime.ContainerListEntry{
		Status: "stopped",
	}
	stopped.Configuration.ID = "ned-a"
	stopped.Configuration.Image.Reference = "docker.io/library/ubuntu:latest"
	stopped.Configuration.Labels = map[string]string{
		runtime.ContainerLabelKey: "1",
		runtime.LabelProfileKey:   "a",
		runtime.LabelImageKey:     "custom-image-a",
		runtime.LabelHostnameKey:  "host-a",
	}

	unmanaged := runtime.ContainerListEntry{
		Status: "running",
	}
	unmanaged.Configuration.ID = "other"
	unmanaged.Configuration.Labels = map[string]string{
		runtime.LabelProfileKey: "ignore-me",
	}

	entries := []runtime.ContainerListEntry{running, stopped, unmanaged}

	runningOnly := buildListRows(entries, false)
	if len(runningOnly) != 1 {
		t.Fatalf("expected 1 running row, got %d", len(runningOnly))
	}
	if runningOnly[0].Name != "ned-b" {
		t.Fatalf("expected running row name ned-b, got %q", runningOnly[0].Name)
	}
	if runningOnly[0].Image != "docker.io/library/alpine:latest" {
		t.Fatalf("expected image fallback to reference, got %q", runningOnly[0].Image)
	}

	allRows := buildListRows(entries, true)
	if len(allRows) != 2 {
		t.Fatalf("expected 2 rows when includeStopped, got %d", len(allRows))
	}
	if allRows[0].Name != "ned-a" || allRows[1].Name != "ned-b" {
		t.Fatalf("expected sorted rows ned-a, ned-b, got %q, %q", allRows[0].Name, allRows[1].Name)
	}
	if allRows[0].Image != "custom-image-a" {
		t.Fatalf("expected image from label, got %q", allRows[0].Image)
	}
	if allRows[0].Workspace != "-" {
		t.Fatalf("expected missing workspace to be '-', got %q", allRows[0].Workspace)
	}
}
