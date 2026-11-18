package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestParseContainerSemver(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  containerSemver
		ok    bool
	}{
		{
			name:  "long form",
			input: "container version 0.10.0",
			want:  containerSemver{Major: 0, Minor: 10, Patch: 0},
			ok:    true,
		},
		{
			name:  "short form",
			input: "0.10.2",
			want:  containerSemver{Major: 0, Minor: 10, Patch: 2},
			ok:    true,
		},
		{
			name:  "invalid",
			input: "container dev build",
			ok:    false,
		},
	}

	for _, tc := range cases {
		got, ok := parseContainerSemver(tc.input)
		if ok != tc.ok {
			t.Fatalf("%s: parseContainerSemver ok=%v want %v", tc.name, ok, tc.ok)
		}
		if !tc.ok {
			continue
		}
		if got != tc.want {
			t.Fatalf("%s: parseContainerSemver got %+v want %+v", tc.name, got, tc.want)
		}
	}
}

func TestRequireContainerRuntimeVersion(t *testing.T) {
	t.Run("supports required version", func(t *testing.T) {
		restore := saveStatusDeps()
		defer restore()

		statusCommandOutput = func(_ string, args ...string) ([]byte, error) {
			if len(args) == 1 && args[0] == "--version" {
				return []byte("container version 1.6.0\n"), nil
			}
			return nil, errors.New("unexpected command")
		}

		if err := RequireContainerRuntimeVersion("/usr/local/bin/container"); err != nil {
			t.Fatalf("RequireContainerRuntimeVersion error: %v", err)
		}
	})

	t.Run("fails on older runtime with upgrade guidance", func(t *testing.T) {
		restore := saveStatusDeps()
		defer restore()

		statusCommandOutput = func(_ string, args ...string) ([]byte, error) {
			if len(args) == 1 && args[0] == "--version" {
				return []byte("container version 1.5.9\n"), nil
			}
			return nil, errors.New("unexpected command")
		}

		err := RequireContainerRuntimeVersion("/usr/local/bin/container")
		if err == nil {
			t.Fatalf("expected version gate error")
		}
		if !strings.Contains(err.Error(), "requires container 1.6.0 or newer") {
			t.Fatalf("expected minimum version message, got: %v", err)
		}
		if !strings.Contains(err.Error(), "Upgrade with `ned runtime install`") {
			t.Fatalf("expected upgrade guidance, got: %v", err)
		}
	})

	t.Run("fails when version command fails", func(t *testing.T) {
		restore := saveStatusDeps()
		defer restore()

		statusCommandOutput = func(_ string, args ...string) ([]byte, error) {
			return nil, errors.New("command failed")
		}

		err := RequireContainerRuntimeVersion("/usr/local/bin/container")
		if err == nil {
			t.Fatalf("expected version detection error")
		}
		if !strings.Contains(err.Error(), "failed to detect version") {
			t.Fatalf("expected detection message, got: %v", err)
		}
	})

	t.Run("fails when version cannot be parsed", func(t *testing.T) {
		restore := saveStatusDeps()
		defer restore()

		statusCommandOutput = func(_ string, args ...string) ([]byte, error) {
			if len(args) == 1 && args[0] == "--version" {
				return []byte("container version dev\n"), nil
			}
			return nil, errors.New("unexpected command")
		}

		err := RequireContainerRuntimeVersion("/usr/local/bin/container")
		if err == nil {
			t.Fatalf("expected parse error")
		}
		if !strings.Contains(err.Error(), "unable to parse version") {
			t.Fatalf("expected parse message, got: %v", err)
		}
	})
}

func TestCheckContainerVersion(t *testing.T) {
	min := minimumSupportedContainerVersion
	below := containerSemver{Major: min.Major, Minor: min.Minor - 1, Patch: 9}
	for _, tc := range []struct {
		output  string
		wantErr string
	}{
		{output: "container CLI version " + min.String() + " (build: release)"},
		{output: "container CLI version 9.0.0"},
		{output: "container CLI version " + below.String(), wantErr: "requires container " + min.String() + " or newer; detected " + below.String()},
		{output: "container CLI version dev", wantErr: "unable to parse version"},
	} {
		err := checkContainerVersion(tc.output)
		if tc.wantErr == "" {
			if err != nil {
				t.Fatalf("checkContainerVersion(%q) error: %v", tc.output, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("checkContainerVersion(%q) = %v, want %q", tc.output, err, tc.wantErr)
		}
	}
}

func TestMissingRequiredDependenciesIncludesOutdated(t *testing.T) {
	state := DependenciesState{Dependencies: []DependencyState{
		{Name: "container", Required: true, Installed: true, Outdated: true},
		{Name: "other", Required: true, Installed: true},
		{Name: "optional", Required: false, Installed: false},
	}}
	got := MissingRequiredDependencies(state)
	if len(got) != 1 || got[0].Name != "container" {
		t.Fatalf("expected only the outdated container runtime, got %+v", got)
	}
}
