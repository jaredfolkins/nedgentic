package frontend

import (
	"strings"
	"testing"

	"nedgentic/internal/profile"
)

func TestValidateLaunchFlagsAllowsContainerSystemd(t *testing.T) {
	flags := profile.DefaultFlags()
	enabled := true
	flags.Systemd = &enabled
	if err := validateLaunchFlags(flags); err != nil {
		t.Fatalf("validateLaunchFlags error: %v", err)
	}
}

func TestValidateLaunchFlagsRejectsInitConflicts(t *testing.T) {
	flags := profile.DefaultFlags()
	systemd := true
	init := true
	flags.Systemd = &systemd
	flags.Init = &init
	err := validateLaunchFlags(flags)
	if err == nil {
		t.Fatal("expected systemd/init conflict to fail")
	}
	if !strings.Contains(err.Error(), "--systemd cannot be combined with --init") {
		t.Fatalf("unexpected error: %v", err)
	}

	flags.Init = nil
	flags.InitImage = "docker.io/library/custom-init:latest"
	err = validateLaunchFlags(flags)
	if err == nil {
		t.Fatal("expected systemd/init-image conflict to fail")
	}
	if !strings.Contains(err.Error(), "--systemd cannot be combined with --init-image") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintContainerStartupLogsTrimsAndWritesFetchedLogs(t *testing.T) {
	var out strings.Builder
	ok := printContainerStartupLogs(&out, func(bin, name string) (string, error) {
		if bin != "container" {
			t.Fatalf("unexpected bin: %q", bin)
		}
		if name != "vm-name" {
			t.Fatalf("unexpected container name: %q", name)
		}
		return "\nned-agent-update: started\n\n", nil
	}, "container", "vm-name")

	if !ok {
		t.Fatal("expected startup logs to be printed")
	}
	if got, want := out.String(), "ned-agent-update: started\n"; got != want {
		t.Fatalf("unexpected logs:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestPrintContainerStartupLogsSkipsEmptyLogs(t *testing.T) {
	var out strings.Builder
	ok := printContainerStartupLogs(&out, func(_, _ string) (string, error) {
		return " \n\t", nil
	}, "container", "vm-name")

	if ok {
		t.Fatal("did not expect empty startup logs to be printed")
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output: %q", out.String())
	}
}
