package runtime

import "testing"

func TestParseSystemDiskUsage(t *testing.T) {
	input := []byte(`{
  "containers": {"active": 1, "reclaimable": 2, "sizeInBytes": 3, "total": 4},
  "images": {"active": 5, "reclaimable": 6, "sizeInBytes": 7, "total": 8},
  "volumes": {"active": 9, "reclaimable": 10, "sizeInBytes": 11, "total": 12}
}`)
	usage, err := parseSystemDiskUsage(input)
	if err != nil {
		t.Fatalf("parseSystemDiskUsage failed: %v", err)
	}
	if usage.Containers.Active != 1 || usage.Containers.ReclaimableBytes != 2 || usage.Containers.SizeBytes != 3 || usage.Containers.Total != 4 {
		t.Fatalf("unexpected containers usage: %+v", usage.Containers)
	}
	if usage.Images.Active != 5 || usage.Images.ReclaimableBytes != 6 || usage.Images.SizeBytes != 7 || usage.Images.Total != 8 {
		t.Fatalf("unexpected images usage: %+v", usage.Images)
	}
	if usage.Volumes.Active != 9 || usage.Volumes.ReclaimableBytes != 10 || usage.Volumes.SizeBytes != 11 || usage.Volumes.Total != 12 {
		t.Fatalf("unexpected volumes usage: %+v", usage.Volumes)
	}
}

func TestParseContainerDataRoot(t *testing.T) {
	output := []byte(`{
  "status": "running",
  "paths": {
    "appRoot": "/Users/jf/Library/Application Support/com.apple.container/",
    "installRoot": "/opt/homebrew/Cellar/container/1.6.0/"
  }
}`)
	root, err := parseContainerDataRoot(output)
	if err != nil {
		t.Fatalf("parseContainerDataRoot failed: %v", err)
	}
	if root != "/Users/jf/Library/Application Support/com.apple.container/" {
		t.Fatalf("unexpected data root: %s", root)
	}
}

func TestParseContainerDataRootRejectsEmptyAppRootField(t *testing.T) {
	output := []byte(`{"status":"running","paths":{"appRoot":""}}`)
	if _, err := parseContainerDataRoot(output); err == nil {
		t.Fatalf("expected empty appRoot to be rejected")
	}
}

func TestParseContainerDataRootRejectsMissingPaths(t *testing.T) {
	output := []byte(`{"status":"running"}`)
	if _, err := parseContainerDataRoot(output); err == nil {
		t.Fatalf("expected missing paths.appRoot to be rejected")
	}
}

func TestParseContainerDataRootRejectsLegacyStatusText(t *testing.T) {
	output := []byte("FIELD VALUE\nappRoot /tmp/container\n")
	if _, err := parseContainerDataRoot(output); err == nil {
		t.Fatalf("expected legacy status text to be rejected")
	}
}

func TestParseContainerDataRootRejectsMalformedSuffix(t *testing.T) {
	output := []byte(`{"status":"running","paths":{"appRoot":"/Users/jf/Library/Application Support/com.apple.container/#frag"}}`)
	if _, err := parseContainerDataRoot(output); err == nil {
		t.Fatalf("expected malformed runtime data root to be rejected")
	}
}

func TestParseDFOutput(t *testing.T) {
	output := "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/disk3s1s1  100  40  60  40% /System/Volumes/Data\n"
	usage, err := parseDFOutput(output)
	if err != nil {
		t.Fatalf("parseDFOutput failed: %v", err)
	}
	if usage.SizeBytes != 100*1024 {
		t.Fatalf("unexpected size bytes: %d", usage.SizeBytes)
	}
	if usage.UsedBytes != 40*1024 {
		t.Fatalf("unexpected used bytes: %d", usage.UsedBytes)
	}
	if usage.FreeBytes != 60*1024 {
		t.Fatalf("unexpected free bytes: %d", usage.FreeBytes)
	}
	if usage.Mount != "/System/Volumes/Data" {
		t.Fatalf("unexpected mount: %s", usage.Mount)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:          "0B",
		512:        "512B",
		1024:       "1.0KiB",
		1536:       "1.5KiB",
		1048576:    "1.0MiB",
		1073741824: "1.0GiB",
	}
	for value, expected := range cases {
		if got := FormatBytes(value); got != expected {
			t.Fatalf("FormatBytes(%d) = %q, want %q", value, got, expected)
		}
	}
}
