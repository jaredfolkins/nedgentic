package frontend

import (
	"reflect"
	"testing"

	"nedgentic/internal/runtime"
)

func TestGenticCommandHerdrFlagWiring(t *testing.T) {
	if genticCmd.PersistentFlags().Lookup("herdr") == nil {
		t.Fatal("expected --herdr flag on gentic command")
	}
}

func TestRunningInsideHerdrRecognizesChildMarkerAndPane(t *testing.T) {
	restore := saveFrontendHerdrDeps()
	defer restore()

	values := map[string]string{herdrChildEnv: "1"}
	herdrGetenv = func(key string) string { return values[key] }
	if !runningInsideHerdr() {
		t.Fatal("expected child marker to identify HerdR child")
	}
	values = map[string]string{"HERDR_PANE_ID": "w1:p1"}
	if !runningInsideHerdr() {
		t.Fatal("expected pane ID to identify HerdR child")
	}
	values = map[string]string{}
	if runningInsideHerdr() {
		t.Fatal("did not expect ordinary shell to be identified as HerdR")
	}
}

func TestMaybeRunProfileInHerdrCreatesRunsAndAttaches(t *testing.T) {
	restore := saveFrontendHerdrDeps()
	defer restore()

	flagHerdr = true
	herdrGetenv = func(string) string { return "" }
	herdrIsTerminal = func(int) bool { return true }
	resolveHerdrBin = func(candidate string) (string, error) {
		return "/usr/local/bin/herdr", nil
	}
	serverChecked := false
	ensureHerdrServer = func(bin string) error {
		serverChecked = true
		return nil
	}
	herdrWorkingDirectory = func() (string, error) { return "/Users/example/project", nil }
	var gotLabel string
	var gotEnv map[string]string
	createHerdrWorkspace = func(bin, cwd, label string, env map[string]string) (runtime.HerdrWorkspace, error) {
		gotLabel = label
		gotEnv = env
		return runtime.HerdrWorkspace{WorkspaceID: "w1", PaneID: "w1:p1", TerminalID: "term_1"}, nil
	}
	herdrExecutable = func() (string, error) { return "/tmp/ned", nil }
	herdrArguments = func() []string {
		return []string{"gentic", "personal", "default", "docker-vm", "--herdr"}
	}
	var gotCommand string
	runHerdrPaneCommand = func(bin, paneID, command string) error {
		gotCommand = command
		return nil
	}
	var attached string
	attachHerdrTerminal = func(bin, terminalID string) error {
		attached = terminalID
		return nil
	}
	closeHerdrWorkspace = func(string, string) error {
		t.Fatal("did not expect a launched HerdR workspace to close on detach")
		return nil
	}

	handled, err := maybeRunProfileInHerdr("default", "docker-vm")
	if err != nil {
		t.Fatalf("maybeRunProfileInHerdr error: %v", err)
	}
	if !handled || !serverChecked {
		t.Fatalf("expected handled launch and server check: handled=%v server=%v", handled, serverChecked)
	}
	if gotLabel != "ned-docker-vm" {
		t.Fatalf("unexpected workspace label: %q", gotLabel)
	}
	if !reflect.DeepEqual(gotEnv, map[string]string{herdrChildEnv: "1"}) {
		t.Fatalf("unexpected workspace environment: %#v", gotEnv)
	}
	wantCommand := "exec '/tmp/ned' 'gentic' 'personal' 'default' 'docker-vm' '--herdr'"
	if gotCommand != wantCommand {
		t.Fatalf("unexpected pane command: got %q want %q", gotCommand, wantCommand)
	}
	if attached != "term_1" {
		t.Fatalf("unexpected attached terminal: %q", attached)
	}
}

func TestMaybeRunProfileInHerdrDoesNotRecurse(t *testing.T) {
	restore := saveFrontendHerdrDeps()
	defer restore()

	flagHerdr = true
	herdrGetenv = func(key string) string {
		if key == "HERDR_PANE_ID" {
			return "w1:p1"
		}
		return ""
	}
	handled, err := maybeRunProfileInHerdr("default", "")
	if err != nil || handled {
		t.Fatalf("expected child process to continue ordinary launch, handled=%v err=%v", handled, err)
	}
}

func saveFrontendHerdrDeps() func() {
	oldFlagHerdr := flagHerdr
	oldResolve := resolveHerdrBin
	oldEnsure := ensureHerdrServer
	oldCreate := createHerdrWorkspace
	oldRun := runHerdrPaneCommand
	oldAttach := attachHerdrTerminal
	oldClose := closeHerdrWorkspace
	oldBuild := buildHerdrExecCommand
	oldExecutable := herdrExecutable
	oldWorkingDirectory := herdrWorkingDirectory
	oldArguments := herdrArguments
	oldGetenv := herdrGetenv
	oldIsTerminal := herdrIsTerminal
	return func() {
		flagHerdr = oldFlagHerdr
		resolveHerdrBin = oldResolve
		ensureHerdrServer = oldEnsure
		createHerdrWorkspace = oldCreate
		runHerdrPaneCommand = oldRun
		attachHerdrTerminal = oldAttach
		closeHerdrWorkspace = oldClose
		buildHerdrExecCommand = oldBuild
		herdrExecutable = oldExecutable
		herdrWorkingDirectory = oldWorkingDirectory
		herdrArguments = oldArguments
		herdrGetenv = oldGetenv
		herdrIsTerminal = oldIsTerminal
	}
}
