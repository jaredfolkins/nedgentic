package frontend

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/runtime"
)

const herdrChildEnv = "NED_HERDR_CHILD"

var (
	resolveHerdrBin       = runtime.ResolveHerdrBin
	ensureHerdrServer     = runtime.EnsureHerdrServer
	createHerdrWorkspace  = runtime.CreateHerdrWorkspace
	runHerdrPaneCommand   = runtime.RunHerdrPaneCommand
	attachHerdrTerminal   = runtime.AttachHerdrTerminal
	closeHerdrWorkspace   = runtime.CloseHerdrWorkspace
	buildHerdrExecCommand = runtime.HerdrExecCommand
	herdrExecutable       = os.Executable
	herdrWorkingDirectory = os.Getwd
	herdrArguments        = func() []string { return append([]string(nil), os.Args[1:]...) }
	herdrGetenv           = os.Getenv
	herdrIsTerminal       = term.IsTerminal
)

func maybeRunProfileInHerdr(profileName, vmNameOverride string) (bool, error) {
	if !flagHerdr || runningInsideHerdr() {
		return false, nil
	}
	if !herdrIsTerminal(int(os.Stdin.Fd())) || !herdrIsTerminal(int(os.Stdout.Fd())) {
		return true, fmt.Errorf("--herdr requires an interactive terminal")
	}

	vmName := strings.TrimSpace(vmNameOverride)
	if vmName == "" {
		var err error
		vmName, err = runtime.ContainerNameForProfile(profileName)
		if err != nil {
			return true, err
		}
	}
	bin, err := resolveHerdrBin("")
	if err != nil {
		return true, err
	}
	if err := runWithSpinner("Start HerdR:", func() error {
		return ensureHerdrServer(bin)
	}); err != nil {
		return true, err
	}

	cwd, err := herdrWorkingDirectory()
	if err != nil {
		return true, fmt.Errorf("resolve HerdR workspace directory: %w", err)
	}
	var workspace runtime.HerdrWorkspace
	if err := runWithSpinner("Create HerdR workspace:", func() error {
		var createErr error
		workspace, createErr = createHerdrWorkspace(bin, cwd, "ned-"+vmName, map[string]string{
			herdrChildEnv: "1",
		})
		return createErr
	}); err != nil {
		return true, err
	}

	started := false
	defer func() {
		if started {
			return
		}
		if closeErr := closeHerdrWorkspace(bin, workspace.WorkspaceID); closeErr != nil {
			logging.Warnf("Failed to close HerdR workspace %s: %v", workspace.WorkspaceID, closeErr)
		}
	}()

	executable, err := herdrExecutable()
	if err != nil {
		return true, fmt.Errorf("resolve Ned executable for HerdR: %w", err)
	}
	command, err := buildHerdrExecCommand(executable, herdrArguments())
	if err != nil {
		return true, err
	}
	if err := runWithSpinner("Launch shell in HerdR:", func() error {
		return runHerdrPaneCommand(bin, workspace.PaneID, command)
	}); err != nil {
		return true, err
	}
	started = true

	output.Printf("Attached via HerdR: workspace=%s vm=%s", workspace.WorkspaceID, vmName)
	if err := attachHerdrTerminal(bin, workspace.TerminalID); err != nil {
		return true, err
	}
	output.Printf("Detached from HerdR: workspace=%s vm=%s", workspace.WorkspaceID, vmName)
	return true, nil
}

func runningInsideHerdr() bool {
	return strings.TrimSpace(herdrGetenv(herdrChildEnv)) == "1" || strings.TrimSpace(herdrGetenv("HERDR_PANE_ID")) != ""
}
