package profile

import "testing"

func TestStagingVMDirValidation(t *testing.T) {
	home := "/home/test"
	if _, err := StagingVMDir(home, ""); err == nil {
		t.Fatalf("expected error for empty vm name")
	}
	if _, err := StagingVMDir(home, "../bad"); err == nil {
		t.Fatalf("expected error for traversal vm name")
	}
	if _, err := StagingVMDir(home, "bad/name"); err == nil {
		t.Fatalf("expected error for path separator vm name")
	}
	got, err := StagingVMDir(home, "vm-name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/home/test/.ned/staging/vm-name"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
