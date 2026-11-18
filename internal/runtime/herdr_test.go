package runtime

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestResolveHerdrBinRequiresInstalledBinary(t *testing.T) {
	restore := saveHerdrDeps()
	defer restore()

	herdrLookPath = func(name string) (string, error) {
		if name != HerdrBinary {
			t.Fatalf("unexpected binary lookup: %q", name)
		}
		return "", exec.ErrNotFound
	}
	_, err := ResolveHerdrBin("")
	if err == nil || !strings.Contains(err.Error(), HerdrInstallURL) {
		t.Fatalf("expected HerdR install guidance, got %v", err)
	}
}

func TestEnsureHerdrServerStartsAndWaitsForReadiness(t *testing.T) {
	restore := saveHerdrDeps()
	defer restore()

	statusCalls := 0
	herdrCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name != "/usr/local/bin/herdr" || !reflect.DeepEqual(args, []string{"status", "server"}) {
			t.Fatalf("unexpected command: %s %v", name, args)
		}
		statusCalls++
		if statusCalls < 3 {
			return []byte("status: not running\n"), nil
		}
		return []byte("status: running\nversion: 0.8.0\n"), nil
	}
	started := false
	herdrStartServer = func(bin string) error {
		if bin != "/usr/local/bin/herdr" {
			t.Fatalf("unexpected binary: %q", bin)
		}
		started = true
		return nil
	}
	herdrSleep = func(time.Duration) {}

	if err := EnsureHerdrServer("/usr/local/bin/herdr"); err != nil {
		t.Fatalf("EnsureHerdrServer error: %v", err)
	}
	if !started {
		t.Fatal("expected server start")
	}
	if statusCalls != 3 {
		t.Fatalf("unexpected status call count: %d", statusCalls)
	}
}

func TestCreateHerdrWorkspaceParsesStableIDs(t *testing.T) {
	restore := saveHerdrDeps()
	defer restore()

	var gotArgs []string
	herdrCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name != "/usr/local/bin/herdr" {
			t.Fatalf("unexpected binary: %q", name)
		}
		gotArgs = append([]string(nil), args...)
		return []byte(`{"result":{"root_pane":{"pane_id":"w1:p1","terminal_id":"term_1"},"workspace":{"workspace_id":"w1"}}}`), nil
	}

	workspace, err := CreateHerdrWorkspace(
		"/usr/local/bin/herdr",
		"/Users/example/project",
		"ned-demo",
		map[string]string{"ZED": "last", "NED_HERDR_CHILD": "1"},
	)
	if err != nil {
		t.Fatalf("CreateHerdrWorkspace error: %v", err)
	}
	want := HerdrWorkspace{WorkspaceID: "w1", PaneID: "w1:p1", TerminalID: "term_1"}
	if workspace != want {
		t.Fatalf("unexpected workspace: got %+v want %+v", workspace, want)
	}
	wantArgs := []string{
		"workspace", "create",
		"--cwd", "/Users/example/project",
		"--label", "ned-demo",
		"--env", "NED_HERDR_CHILD=1",
		"--env", "ZED=last",
		"--focus",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("unexpected HerdR args: got %#v want %#v", gotArgs, wantArgs)
	}
}

func TestCreateHerdrWorkspaceRejectsMissingIDs(t *testing.T) {
	restore := saveHerdrDeps()
	defer restore()

	herdrCommandOutput = func(string, ...string) ([]byte, error) {
		return []byte(`{"result":{"root_pane":{},"workspace":{}}}`), nil
	}
	_, err := CreateHerdrWorkspace("herdr", "/tmp/project", "ned-demo", nil)
	if err == nil || !strings.Contains(err.Error(), "omitted workspace, pane, or terminal ID") {
		t.Fatalf("expected missing-ID error, got %v", err)
	}
}

func TestHerdrExecCommandQuotesEveryArgument(t *testing.T) {
	got, err := HerdrExecCommand("/Applications/Ned Tools/ned", []string{"gentic", "it's-safe", ""})
	if err != nil {
		t.Fatalf("HerdrExecCommand error: %v", err)
	}
	want := `exec '/Applications/Ned Tools/ned' 'gentic' 'it'"'"'s-safe' ''`
	if got != want {
		t.Fatalf("unexpected command:\n got: %s\nwant: %s", got, want)
	}
}

func TestHerdrCommandHelpersUseStructuredArguments(t *testing.T) {
	restore := saveHerdrDeps()
	defer restore()

	var commands [][]string
	herdrCommandOutput = func(name string, args ...string) ([]byte, error) {
		commands = append(commands, append([]string{name}, args...))
		return []byte(`{"result":{"type":"ok"}}`), nil
	}
	herdrRunCommand = func(name string, args ...string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}
	if err := RunHerdrPaneCommand("herdr", "w1:p1", "exec 'ned'"); err != nil {
		t.Fatalf("RunHerdrPaneCommand error: %v", err)
	}
	if err := AttachHerdrTerminal("herdr", "term_1"); err != nil {
		t.Fatalf("AttachHerdrTerminal error: %v", err)
	}
	if err := CloseHerdrWorkspace("herdr", "w1"); err != nil {
		t.Fatalf("CloseHerdrWorkspace error: %v", err)
	}
	want := [][]string{
		{"herdr", "pane", "run", "w1:p1", "exec 'ned'"},
		{"herdr", "terminal", "attach", "term_1"},
		{"herdr", "workspace", "close", "w1"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected commands: got %#v want %#v", commands, want)
	}
}

func TestHerdrServerStatusErrorIncludesOutput(t *testing.T) {
	restore := saveHerdrDeps()
	defer restore()

	herdrCommandOutput = func(string, ...string) ([]byte, error) {
		return []byte("protocol mismatch\n"), errors.New("exit status 1")
	}
	_, err := HerdrServerRunning("herdr")
	if err == nil || !strings.Contains(err.Error(), "protocol mismatch") {
		t.Fatalf("expected command output in error, got %v", err)
	}
}

func saveHerdrDeps() func() {
	oldLookPath := herdrLookPath
	oldCommandOutput := herdrCommandOutput
	oldRunCommand := herdrRunCommand
	oldStartServer := herdrStartServer
	oldSleep := herdrSleep
	return func() {
		herdrLookPath = oldLookPath
		herdrCommandOutput = oldCommandOutput
		herdrRunCommand = oldRunCommand
		herdrStartServer = oldStartServer
		herdrSleep = oldSleep
	}
}
