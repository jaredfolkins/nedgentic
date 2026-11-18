package frontend

import (
	"testing"

	"nedgentic/internal/backend/runtime"
)

func TestBuildStorageRows(t *testing.T) {
	entries := []runtime.VolumeEntry{
		{Name: "ned-disk-alpha", SizeBytes: 1024, UsedBytes: 512, Source: "/tmp/alpha"},
		{Name: "other-volume", SizeBytes: 2048, UsedBytes: 1024, Source: ""},
	}

	rows := buildStorageRows(entries, false)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Name != "ned-disk-alpha" {
		t.Fatalf("unexpected row name: %s", rows[0].Name)
	}
	if rows[0].Size != "1.0KiB" {
		t.Fatalf("unexpected size: %s", rows[0].Size)
	}
	if rows[0].Used != "512B" {
		t.Fatalf("unexpected used: %s", rows[0].Used)
	}
	if rows[0].Source != "/tmp/alpha" {
		t.Fatalf("unexpected source: %s", rows[0].Source)
	}

	rows = buildStorageRows(entries, true)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}
