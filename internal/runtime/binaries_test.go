package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateHostBinaryELF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "elfbin")
	if err := os.WriteFile(path, []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01}, 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := validateHostBinary(path); err != nil {
		t.Fatalf("expected ELF to validate, got: %v", err)
	}
}

func TestValidateHostBinaryScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho hi\n"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := validateHostBinary(path); err != nil {
		t.Fatalf("expected script to validate, got: %v", err)
	}
}

func TestValidateHostBinaryMachO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mach")
	if err := os.WriteFile(path, []byte{0xfe, 0xed, 0xfa, 0xcf, 0x00}, 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := validateHostBinary(path); err == nil {
		t.Fatalf("expected Mach-O to fail validation")
	}
}

func TestBinaryMountFromSpec(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho ok\n"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	mount, err := binaryMountFromSpec(path)
	if err != nil {
		t.Fatalf("binaryMountFromSpec error: %v", err)
	}
	if mount == nil {
		t.Fatalf("expected mount")
	}
	if mount.HostPath != path {
		t.Fatalf("unexpected HostPath: %s", mount.HostPath)
	}
	expected := filepath.Join("/usr/local/bin", filepath.Base(path))
	if mount.VMPath != expected {
		t.Fatalf("unexpected VMPath: %s", mount.VMPath)
	}
}
