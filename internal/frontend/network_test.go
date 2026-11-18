package frontend

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"nedgentic/internal/profile"
)

func TestResolveNetworkContainerBinRejectsEmptyPath(t *testing.T) {
	origResolve := networkResolveContainerBin
	defer func() {
		networkResolveContainerBin = origResolve
	}()

	networkResolveContainerBin = func(flags profile.Flags) (string, error) {
		return "", nil
	}

	_, err := resolveNetworkContainerBin(&cobra.Command{Use: "test"})
	if err == nil {
		t.Fatal("expected error for empty bin")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNetworkArgBuilders(t *testing.T) {
	if got := networkCreateArgs("ned-lab", false); !reflect.DeepEqual(got, []string{"network", "create", "ned-lab"}) {
		t.Fatalf("unexpected create args: %#v", got)
	}
	if got := networkCreateArgs("ned-lab", true); !reflect.DeepEqual(got, []string{"network", "create", "--internal", "ned-lab"}) {
		t.Fatalf("unexpected internal create args: %#v", got)
	}
	if got := networkRemoveArgs("ned-lab"); !reflect.DeepEqual(got, []string{"network", "delete", "ned-lab"}) {
		t.Fatalf("unexpected remove args: %#v", got)
	}
}

func TestRunNetworkCommandIncludesCLIOutputOnError(t *testing.T) {
	origOutput := networkCommandOutput
	defer func() {
		networkCommandOutput = origOutput
	}()

	networkCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("bad request"), errors.New("exit status 1")
	}

	_, err := runNetworkCommand("/usr/local/bin/container", "network", "create", "bad")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("expected command output in error, got: %v", err)
	}
}

func TestRunNetworkCommandSanitizesCLIOutputOnError(t *testing.T) {
	origOutput := networkCommandOutput
	defer func() {
		networkCommandOutput = origOutput
	}()

	networkCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("\x1b[31mbad request\x1b[0m\x00"), errors.New("exit status 1")
	}

	_, err := runNetworkCommand("/usr/local/bin/container", "network", "create", "bad")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "\x1b") || strings.Contains(err.Error(), "\x00") {
		t.Fatalf("expected sanitized error, got: %q", err.Error())
	}
}

func TestNetworkCommandFlagWiring(t *testing.T) {
	if networkCreateCmd.Flags().Lookup("internal") == nil {
		t.Fatalf("expected --internal flag on network create command")
	}
	if len(networkRemoveCmd.Aliases) == 0 {
		t.Fatalf("expected remove aliases")
	}
	foundRM := slices.Contains(networkRemoveCmd.Aliases, "rm")
	if !foundRM {
		t.Fatalf("expected rm alias on network remove command")
	}
}

func TestNetworkCreateCommandRunAssemblesArgs(t *testing.T) {
	origResolve := networkResolveContainerBin
	origOutput := networkCommandOutput
	origInternal := networkCreateInternal
	defer func() {
		networkResolveContainerBin = origResolve
		networkCommandOutput = origOutput
		networkCreateInternal = origInternal
	}()

	networkResolveContainerBin = func(flags profile.Flags) (string, error) {
		return "/usr/local/bin/container", nil
	}

	var gotName string
	var gotArgs []string
	networkCommandOutput = func(name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte("created"), nil
	}

	networkCreateInternal = true
	networkCreateCmd.Run(networkCreateCmd, []string{"team-internal"})

	if gotName != "/usr/local/bin/container" {
		t.Fatalf("unexpected command binary: %q", gotName)
	}
	want := []string{"network", "create", "--internal", "team-internal"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("unexpected command args: got %#v want %#v", gotArgs, want)
	}
}
