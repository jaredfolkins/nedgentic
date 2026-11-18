package frontend

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/profile"
)

func TestEnsureDependenciesStartsContainerRuntimeServiceBeforeWritingState(t *testing.T) {
	origResolveHostUser := runtime.ResolveHostUser
	origCheckDependencies := runtime.CheckDependencies
	origInstallDependency := runtime.InstallDependency
	origWriteDependenciesState := runtime.WriteDependenciesState
	origContainerRuntimeServiceRunning := runtime.ContainerRuntimeServiceRunning
	origStartContainerRuntimeService := runtime.StartContainerRuntimeService
	origCheckHostPlatform := runtime.CheckHostPlatform
	runtime.CheckHostPlatform = func() error { return nil }
	defer func() {
		runtime.CheckHostPlatform = origCheckHostPlatform
		runtime.ResolveHostUser = origResolveHostUser
		runtime.CheckDependencies = origCheckDependencies
		runtime.InstallDependency = origInstallDependency
		runtime.WriteDependenciesState = origWriteDependenciesState
		runtime.ContainerRuntimeServiceRunning = origContainerRuntimeServiceRunning
		runtime.StartContainerRuntimeService = origStartContainerRuntimeService
	}()

	home := t.TempDir()
	state := runtime.DependenciesState{
		Dependencies: []runtime.DependencyState{
			{Name: "container", Required: true, Installed: true, Path: "/usr/local/bin/container"},
		},
	}

	runtime.ResolveHostUser = func() (*runtime.HostUser, error) {
		return &runtime.HostUser{Name: "tester", UID: 501, GID: 20, Home: home}, nil
	}
	runtime.CheckDependencies = func(flags profile.Flags) (runtime.DependenciesState, error) {
		return state, nil
	}
	runtime.InstallDependency = func(name string, out io.Writer) error {
		t.Fatalf("did not expect dependency installation during this test")
		return nil
	}

	var steps []string
	runtime.ContainerRuntimeServiceRunning = func(bin string) (bool, error) {
		if bin != "/usr/local/bin/container" {
			t.Fatalf("unexpected container runtime path: %q", bin)
		}
		steps = append(steps, "status")
		return false, nil
	}
	runtime.StartContainerRuntimeService = func(bin string) error {
		if bin != "/usr/local/bin/container" {
			t.Fatalf("unexpected container runtime path for start: %q", bin)
		}
		steps = append(steps, "start")
		return nil
	}
	runtime.WriteDependenciesState = func(gotHome string, gotState runtime.DependenciesState) error {
		if gotHome != home {
			t.Fatalf("unexpected home path: got %q want %q", gotHome, home)
		}
		if !reflect.DeepEqual(gotState, state) {
			t.Fatalf("unexpected dependency state: got %+v want %+v", gotState, state)
		}
		steps = append(steps, "write")
		return nil
	}

	cmd := &cobra.Command{Use: "test"}
	if err := ensureDependencies(cmd, profile.DefaultFlags()); err != nil {
		t.Fatalf("ensureDependencies error: %v", err)
	}

	want := []string{"status", "start", "write"}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("unexpected call order: got %v want %v", steps, want)
	}
}

func stubDependencyRuntime(t *testing.T) {
	t.Helper()
	origs := []func(){}
	save := func(restore func()) { origs = append(origs, restore) }
	origResolveHostUser := runtime.ResolveHostUser
	save(func() { runtime.ResolveHostUser = origResolveHostUser })
	origCheckDependencies := runtime.CheckDependencies
	save(func() { runtime.CheckDependencies = origCheckDependencies })
	origInstallDependency := runtime.InstallDependency
	save(func() { runtime.InstallDependency = origInstallDependency })
	origWrite := runtime.WriteDependenciesState
	save(func() { runtime.WriteDependenciesState = origWrite })
	origRunning := runtime.ContainerRuntimeServiceRunning
	save(func() { runtime.ContainerRuntimeServiceRunning = origRunning })
	origStart := runtime.StartContainerRuntimeService
	save(func() { runtime.StartContainerRuntimeService = origStart })
	origHost := runtime.CheckHostPlatform
	save(func() { runtime.CheckHostPlatform = origHost })
	t.Cleanup(func() {
		for _, restore := range origs {
			restore()
		}
	})

	home := t.TempDir()
	runtime.ResolveHostUser = func() (*runtime.HostUser, error) {
		return &runtime.HostUser{Name: "tester", UID: 501, GID: 20, Home: home}, nil
	}
	runtime.CheckHostPlatform = func() error { return nil }
	runtime.WriteDependenciesState = func(string, runtime.DependenciesState) error { return nil }
	runtime.ContainerRuntimeServiceRunning = func(string) (bool, error) { return true, nil }
	runtime.StartContainerRuntimeService = func(string) error { return nil }
}

func TestEnsureDependenciesUpgradesOutdatedContainerRuntime(t *testing.T) {
	stubDependencyRuntime(t)
	outdated := runtime.DependenciesState{Dependencies: []runtime.DependencyState{
		{Name: "container", Required: true, Installed: true, Outdated: true, Path: "/usr/local/bin/container", Version: "container CLI version 1.5.0"},
	}}
	current := runtime.DependenciesState{Dependencies: []runtime.DependencyState{
		{Name: "container", Required: true, Installed: true, Path: "/usr/local/bin/container", Version: "container CLI version 1.7.1"},
	}}
	checks := 0
	runtime.CheckDependencies = func(profile.Flags) (runtime.DependenciesState, error) {
		checks++
		if checks == 1 {
			return outdated, nil
		}
		return current, nil
	}
	var installed []string
	runtime.InstallDependency = func(name string, _ io.Writer) error {
		installed = append(installed, name)
		return nil
	}

	if err := ensureDependencies(&cobra.Command{Use: "test"}, profile.DefaultFlags()); err != nil {
		t.Fatalf("ensureDependencies error: %v", err)
	}
	if !reflect.DeepEqual(installed, []string{"container"}) {
		t.Fatalf("expected outdated container runtime to be upgraded once, got %v", installed)
	}
	if got := dependencyInstallLabel(outdated.Dependencies[0]); got != "Upgrade container runtime:" {
		t.Fatalf("unexpected upgrade label %q", got)
	}
}

func TestEnsureDependenciesStopsOnUnsupportedHost(t *testing.T) {
	stubDependencyRuntime(t)
	runtime.CheckHostPlatform = func() error { return errors.New("ned requires macOS 26 or later") }
	runtime.CheckDependencies = func(profile.Flags) (runtime.DependenciesState, error) {
		t.Fatalf("did not expect dependency checks on an unsupported host")
		return runtime.DependenciesState{}, nil
	}

	err := ensureDependencies(&cobra.Command{Use: "test"}, profile.DefaultFlags())
	if err == nil || !strings.Contains(err.Error(), "macOS 26") {
		t.Fatalf("expected host platform error, got %v", err)
	}
}
