package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	HerdrBinary     = "herdr"
	HerdrInstallURL = "https://herdr.dev/docs/install/"
)

type HerdrWorkspace struct {
	WorkspaceID string
	PaneID      string
	TerminalID  string
}

type herdrWorkspaceCreateResponse struct {
	Result struct {
		RootPane struct {
			PaneID     string `json:"pane_id"`
			TerminalID string `json:"terminal_id"`
		} `json:"root_pane"`
		Workspace struct {
			WorkspaceID string `json:"workspace_id"`
		} `json:"workspace"`
	} `json:"result"`
}

var (
	herdrLookPath      = exec.LookPath
	herdrCommandOutput = runtimeCommandOutput
	herdrRunCommand    = runCommandStreaming
	herdrStartServer   = startDetachedHerdrServer
	herdrSleep         = time.Sleep
)

func ResolveHerdrBin(candidate string) (string, error) {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		trimmed = HerdrBinary
	}
	path, err := herdrLookPath(trimmed)
	if err != nil {
		return "", fmt.Errorf("HerdR is required for --herdr; install it from %s: %w", HerdrInstallURL, err)
	}
	return path, nil
}

func HerdrServerRunning(bin string) (bool, error) {
	if strings.TrimSpace(bin) == "" {
		return false, fmt.Errorf("HerdR binary is required")
	}
	out, err := herdrCommandOutput(bin, "status", "server")
	if err != nil {
		return false, fmt.Errorf("HerdR server status failed: %w", FormatCommandOutputError(err, out))
	}
	for _, line := range SanitizeStructuredCommandLines(out) {
		if strings.EqualFold(strings.TrimSpace(line), "status: running") {
			return true, nil
		}
	}
	return false, nil
}

func EnsureHerdrServer(bin string) error {
	running, statusErr := HerdrServerRunning(bin)
	if statusErr == nil && running {
		return nil
	}
	if err := herdrStartServer(bin); err != nil {
		if statusErr != nil {
			return fmt.Errorf("HerdR server status failed: %v; failed to start server: %w", statusErr, err)
		}
		return fmt.Errorf("failed to start HerdR server: %w", err)
	}

	var lastErr error
	for range 50 {
		running, err := HerdrServerRunning(bin)
		if err == nil && running {
			return nil
		}
		lastErr = err
		herdrSleep(100 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("HerdR server did not become ready: %w", lastErr)
	}
	return fmt.Errorf("HerdR server did not become ready within 5 seconds")
}

func CreateHerdrWorkspace(bin, cwd, label string, env map[string]string) (HerdrWorkspace, error) {
	if strings.TrimSpace(bin) == "" {
		return HerdrWorkspace{}, fmt.Errorf("HerdR binary is required")
	}
	cwd = strings.TrimSpace(cwd)
	if cwd == "" || !filepath.IsAbs(cwd) {
		return HerdrWorkspace{}, fmt.Errorf("HerdR workspace directory must be absolute")
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return HerdrWorkspace{}, fmt.Errorf("HerdR workspace label is required")
	}

	args := []string{"workspace", "create", "--cwd", cwd, "--label", label}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsRune(key, '=') {
			return HerdrWorkspace{}, fmt.Errorf("invalid HerdR environment key %q", key)
		}
		args = append(args, "--env", key+"="+env[key])
	}
	args = append(args, "--focus")

	out, err := herdrCommandOutput(bin, args...)
	if err != nil {
		return HerdrWorkspace{}, fmt.Errorf("create HerdR workspace: %w", FormatCommandOutputError(err, out))
	}
	var response herdrWorkspaceCreateResponse
	if err := json.Unmarshal(out, &response); err != nil {
		return HerdrWorkspace{}, fmt.Errorf("parse HerdR workspace response: %w", err)
	}
	workspace := HerdrWorkspace{
		WorkspaceID: strings.TrimSpace(response.Result.Workspace.WorkspaceID),
		PaneID:      strings.TrimSpace(response.Result.RootPane.PaneID),
		TerminalID:  strings.TrimSpace(response.Result.RootPane.TerminalID),
	}
	if workspace.WorkspaceID == "" || workspace.PaneID == "" || workspace.TerminalID == "" {
		return HerdrWorkspace{}, fmt.Errorf("HerdR workspace response omitted workspace, pane, or terminal ID")
	}
	return workspace, nil
}

func RunHerdrPaneCommand(bin, paneID, command string) error {
	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		return fmt.Errorf("HerdR pane ID is required")
	}
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("HerdR pane command is required")
	}
	out, err := herdrCommandOutput(bin, "pane", "run", paneID, command)
	if err != nil {
		return fmt.Errorf("run command in HerdR pane: %w", FormatCommandOutputError(err, out))
	}
	return nil
}

func AttachHerdrTerminal(bin, terminalID string) error {
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return fmt.Errorf("HerdR terminal ID is required")
	}
	if err := herdrRunCommand(bin, "terminal", "attach", terminalID); err != nil {
		return fmt.Errorf("attach HerdR terminal: %w", err)
	}
	return nil
}

func CloseHerdrWorkspace(bin, workspaceID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil
	}
	out, err := herdrCommandOutput(bin, "workspace", "close", workspaceID)
	if err != nil {
		return fmt.Errorf("close HerdR workspace: %w", FormatCommandOutputError(err, out))
	}
	return nil
}

func HerdrExecCommand(executable string, args []string) (string, error) {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return "", fmt.Errorf("executable is required")
	}
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuoteCommandArg(executable))
	for _, arg := range args {
		parts = append(parts, shellQuoteCommandArg(arg))
	}
	return "exec " + strings.Join(parts, " "), nil
}

func shellQuoteCommandArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func startDetachedHerdrServer(bin string) error {
	cmd := exec.Command(bin, "server")
	cmd.Env = os.Environ()
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
