package runtime

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAppendResourceArgs(t *testing.T) {
	args := appendResourceArgs([]string{"run"}, "", "")
	if !reflect.DeepEqual(args, []string{"run"}) {
		t.Fatalf("expected args to remain unchanged, got %#v", args)
	}

	args = appendResourceArgs([]string{"run"}, "2", "4G")
	want := []string{"run", "--cpus", "2", "--memory", "4G"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestDiskVolumeName(t *testing.T) {
	name, err := diskVolumeName("My VM")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "ned-disk-my-vm" {
		t.Fatalf("unexpected volume name: %s", name)
	}
}

func TestGetContainerRuntimeRootUsesSystemStatus(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "container")
	script := `#!/bin/sh
if [ "$1" = "system" ] && [ "$2" = "status" ] && [ "$3" = "--format" ] && [ "$4" = "json" ]; then
  printf '{"status":"running","paths":{"appRoot":"/tmp/container data root"}}\n'
  exit 0
fi
printf 'unexpected args: %s\n' "$*" >&2
exit 64
`
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatalf("write fake container: %v", err)
	}

	root, err := getContainerRuntimeRoot(bin)
	if err != nil {
		t.Fatalf("getContainerRuntimeRoot failed: %v", err)
	}
	if root != "/tmp/container data root" {
		t.Fatalf("unexpected runtime root: %q", root)
	}
}

func TestSanitizeResourceValue(t *testing.T) {
	if _, err := sanitizeResourceValue("cpus", "1 2"); err == nil {
		t.Fatalf("expected error for spaces")
	}
	if _, err := sanitizeResourceValue("memory", "1,2"); err == nil {
		t.Fatalf("expected error for commas")
	}
	value, err := sanitizeResourceValue("memory", "4G")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "4G" {
		t.Fatalf("unexpected value: %q", value)
	}
}
