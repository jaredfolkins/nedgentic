package runtime

import "testing"

func TestExecContainerShellRequiresHostUser(t *testing.T) {
	err := ExecContainerShell(ExecShellOptions{
		Bin:           "container",
		ContainerName: "vm",
		ShellPath:     "/bin/bash",
		Workdir:       "/tmp",
	})
	if err == nil {
		t.Fatal("expected error when host user is missing")
	}
}
