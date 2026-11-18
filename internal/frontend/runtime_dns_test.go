package frontend

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"nedgentic/internal/profile"
)

func TestResolveRuntimeDNSContainerBinRejectsEmptyPath(t *testing.T) {
	origResolve := runtimeDNSResolveContainerBin
	defer func() {
		runtimeDNSResolveContainerBin = origResolve
	}()

	runtimeDNSResolveContainerBin = func(flags profile.Flags) (string, error) {
		return "", nil
	}

	_, err := resolveRuntimeDNSContainerBin(&cobra.Command{Use: "test"})
	if err == nil {
		t.Fatal("expected error for empty bin")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRuntimeDNSArgBuilders(t *testing.T) {
	if got := runtimeDNSListArgs("", false); !reflect.DeepEqual(got, []string{"system", "dns", "list"}) {
		t.Fatalf("unexpected list args: %#v", got)
	}
	if got := runtimeDNSListArgs("json", true); !reflect.DeepEqual(got, []string{"system", "dns", "list", "--format", "json", "--quiet"}) {
		t.Fatalf("unexpected list args with flags: %#v", got)
	}
	if got := runtimeDNSCreateArgs("containers.local", nil); !reflect.DeepEqual(got, []string{"system", "dns", "create", "containers.local"}) {
		t.Fatalf("unexpected default create args: %#v", got)
	}
	if got := runtimeDNSCreateArgs("containers.local", []string{" host.gateway ", "", "internal.api"}); !reflect.DeepEqual(got, []string{"system", "dns", "create", "--localhost", "host.gateway", "--localhost", "internal.api", "containers.local"}) {
		t.Fatalf("unexpected create args: %#v", got)
	}
	if got := runtimeDNSDeleteArgs("containers.local"); !reflect.DeepEqual(got, []string{"system", "dns", "delete", "containers.local"}) {
		t.Fatalf("unexpected delete args: %#v", got)
	}
}

func TestRunRuntimeDNSCommandIncludesCLIOutputOnError(t *testing.T) {
	origOutput := runtimeDNSCommandOutput
	defer func() {
		runtimeDNSCommandOutput = origOutput
	}()

	runtimeDNSCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("permission denied"), errors.New("exit status 1")
	}

	_, err := runRuntimeDNSCommand("/usr/local/bin/container", "system", "dns", "create", "containers.local")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected command output in error, got: %v", err)
	}
}

func TestRunRuntimeDNSCommandSanitizesCLIOutputOnError(t *testing.T) {
	origOutput := runtimeDNSCommandOutput
	defer func() {
		runtimeDNSCommandOutput = origOutput
	}()

	runtimeDNSCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("\x1b[31mpermission denied\x1b[0m\x00"), errors.New("exit status 1")
	}

	_, err := runRuntimeDNSCommand("/usr/local/bin/container", "system", "dns", "create", "containers.local")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "\x1b") || strings.Contains(err.Error(), "\x00") {
		t.Fatalf("expected sanitized error, got: %q", err.Error())
	}
}

func TestRuntimeDNSCommandFlagWiring(t *testing.T) {
	if runtimeDNSCreateCmd.Flags().Lookup("localhost") == nil {
		t.Fatalf("expected --localhost flag on runtime dns create command")
	}
	if runtimeDNSListCmd.Flags().Lookup("format") == nil {
		t.Fatalf("expected --format flag on runtime dns list command")
	}
	if runtimeDNSListCmd.Flags().Lookup("quiet") == nil {
		t.Fatalf("expected --quiet flag on runtime dns list command")
	}
	if len(runtimeDNSDeleteCmd.Aliases) == 0 {
		t.Fatalf("expected runtime dns delete aliases")
	}
}

func TestRuntimeDNSListCommandRunAssemblesArgs(t *testing.T) {
	origResolve := runtimeDNSResolveContainerBin
	origOutput := runtimeDNSCommandOutput
	origFormat := runtimeDNSListFormat
	origQuiet := runtimeDNSListQuiet
	defer func() {
		runtimeDNSResolveContainerBin = origResolve
		runtimeDNSCommandOutput = origOutput
		runtimeDNSListFormat = origFormat
		runtimeDNSListQuiet = origQuiet
	}()

	runtimeDNSResolveContainerBin = func(flags profile.Flags) (string, error) {
		return "/usr/local/bin/container", nil
	}

	var gotName string
	var gotArgs []string
	runtimeDNSCommandOutput = func(name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte("dev.test"), nil
	}

	runtimeDNSListFormat = "json"
	runtimeDNSListQuiet = true
	runtimeDNSListCmd.Run(runtimeDNSListCmd, nil)

	if gotName != "/usr/local/bin/container" {
		t.Fatalf("unexpected command binary: %q", gotName)
	}
	want := []string{"system", "dns", "list", "--format", "json", "--quiet"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("unexpected command args: got %#v want %#v", gotArgs, want)
	}
}

func TestRuntimeDNSCreateCommandRunAssemblesArgs(t *testing.T) {
	origResolve := runtimeDNSResolveContainerBin
	origOutput := runtimeDNSCommandOutput
	origLocalhost := runtimeDNSCreateLocalhost
	defer func() {
		runtimeDNSResolveContainerBin = origResolve
		runtimeDNSCommandOutput = origOutput
		runtimeDNSCreateLocalhost = origLocalhost
	}()

	runtimeDNSResolveContainerBin = func(flags profile.Flags) (string, error) {
		return "/usr/local/bin/container", nil
	}

	var gotName string
	var gotArgs []string
	runtimeDNSCommandOutput = func(name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte("created"), nil
	}

	runtimeDNSCreateLocalhost = []string{"host.gateway", "internal.api"}
	runtimeDNSCreateCmd.Run(runtimeDNSCreateCmd, []string{"dev.test"})

	if gotName != "/usr/local/bin/container" {
		t.Fatalf("unexpected command binary: %q", gotName)
	}
	want := []string{"system", "dns", "create", "--localhost", "host.gateway", "--localhost", "internal.api", "dev.test"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("unexpected command args: got %#v want %#v", gotArgs, want)
	}
}
