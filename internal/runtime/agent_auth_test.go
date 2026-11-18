package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"nedgentic/internal/profile"
	"nedgentic/internal/scripts"
)

func TestAgentAuthVolumeName(t *testing.T) {
	got, err := AgentAuthVolumeName("work-a")
	if err != nil {
		t.Fatalf("AgentAuthVolumeName: %v", err)
	}
	if got != "ned-auth-work-a" {
		t.Fatalf("volume name = %q, want ned-auth-work-a", got)
	}
	if _, err := AgentAuthVolumeName("  "); err == nil {
		t.Fatalf("expected error for empty container name")
	}
}

func TestBuildRunArgsAgentAuthSetsClaudeConfigDir(t *testing.T) {
	base := BuildRunArgsOptions{
		Image:         "docker.io/library/alpine",
		ContainerName: "vm",
		ProfileName:   "default",
		Hostname:      "vm",
		Flags:         profile.DefaultFlags(),
		Plan:          &MountPlan{},
		HostUser:      &HostUser{Name: "alice"},
		Detached:      true,
	}
	if containsArgPair(buildRunArgs(base), "--env", "CLAUDE_CONFIG_DIR=/home/alice/.claude") {
		t.Fatalf("CLAUDE_CONFIG_DIR set without agent auth volume")
	}
	base.AgentAuth = true
	if !containsArgPair(buildRunArgs(base), "--env", "CLAUDE_CONFIG_DIR=/home/alice/.claude") {
		t.Fatalf("expected CLAUDE_CONFIG_DIR with agent auth volume")
	}
}

func TestRunnerScriptsLinkAgentAuthAndInstallLogin(t *testing.T) {
	user := &HostUser{Name: "alice", UID: 501, GID: 20}
	runner, err := genticRunnerScript(user, "/bin/bash", "", &MountPlan{}, true)
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	bootstrap, err := genticBootstrapScript(user, "/bin/bash", "", &MountPlan{})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	for name, script := range map[string]string{"runner": runner, "bootstrap": bootstrap} {
		for _, want := range []string{
			"NED_AUTH_DIR='/mnt/ned-auth'",
			"for NED_AUTH_ENTRY in .claude .codex .gemini .grok .local/share/opencode; do",
			`if [ -e "$NED_AUTH_TARGET" ] && [ ! -L "$NED_AUTH_TARGET" ]; then continue; fi`,
			"cat > /usr/local/bin/ned-agent-login <<'NED_AGENT_LOGIN_EOF'",
			"\nNED_AGENT_LOGIN_EOF\n",
		} {
			if !strings.Contains(script, want) {
				t.Fatalf("%s script missing %q", name, want)
			}
		}
		if ready := strings.Index(script, "touch /tmp/ned-ready"); ready >= 0 && strings.Index(script, "NED_AUTH_DIR=") > ready {
			t.Fatalf("%s: agent auth block must run before the ready marker", name)
		}
		path := filepath.Join(t.TempDir(), name+".sh")
		if err := os.WriteFile(path, []byte(script), 0600); err != nil {
			t.Fatalf("write script: %v", err)
		}
		if out, err := exec.Command("sh", "-n", path).CombinedOutput(); err != nil {
			t.Fatalf("%s script is not valid sh: %v: %s", name, err, out)
		}
	}
}

func TestShellFallbackRunsAgentLoginOnInteractiveAttach(t *testing.T) {
	script, err := shellScriptWithFallback("/bin/bash", "")
	if err != nil {
		t.Fatalf("shell script: %v", err)
	}
	if !strings.Contains(script, "/usr/local/bin/ned-agent-login --attach || true") {
		t.Fatalf("attach shell does not run the login walkthrough:\n%s", script)
	}
	if !strings.Contains(script, "[ -t 0 ] && [ -t 1 ]") {
		t.Fatalf("login walkthrough must only run on a TTY")
	}
}

func TestAgentLoginScript(t *testing.T) {
	script := scripts.AgentLoginScript()
	path := filepath.Join(t.TempDir(), "ned-agent-login")
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if out, err := exec.Command("bash", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("ned-agent-login is not valid bash: %v: %s", err, out)
	}
	for _, want := range []string{
		"claude auth login",
		"codex login --device-auth",
		"grok login --device-auth",
		"env NO_BROWSER=true gemini",
		"opencode auth login",
		"SKIP_FILE=\"$AUTH_DIR/.ned-login-skipped\"",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("ned-agent-login missing %q", want)
		}
	}
	// Without a TTY the walkthrough must exit quietly so scripted attaches never block.
	cmd := exec.Command("bash", path, "--attach")
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "" {
		t.Fatalf("non-tty run: err=%v out=%q", err, out)
	}
}
