package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	nedprofile "nedgentic/internal/profile"
	nedruntime "nedgentic/internal/runtime"
)

func TestE2EAlpineDefault(t *testing.T) {
	requireContainerRuntime(t)

	repoRoot := repoRoot(t)
	binDir := t.TempDir()
	nedPath := filepath.Join(binDir, "ned")
	buildBinary(t, repoRoot, "./cmd/ned", nedPath)
	vmName := fmt.Sprintf("e2e-alpine-%d", time.Now().UnixNano())
	writeProfile(t, vmName, `{
  "image": "docker.io/library/alpine",
  "flags": { "rm-container-on-exit": true }
}`)
	t.Cleanup(func() {
		cleanupVM(t, nedPath, repoRoot, vmName)
	})
	logVMs(t, nedPath, repoRoot)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, nedPath, "gentic", vmName, vmName)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	cmd.Stdin = strings.NewReader("exit\n")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	t.Logf("CMD: %s", formatCommand(nedPath, "gentic", vmName, vmName))
	err := cmd.Run()
	logCommandOutput(t, output.Bytes())
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("ned gentic timed out")
	}
	if err != nil {
		t.Fatalf("ned gentic failed: %v\n%s", err, output.String())
	}
	if strings.Contains(output.String(), "unknown user") {
		t.Fatalf("unexpected unknown user error:\n%s", output.String())
	}
	logVMs(t, nedPath, repoRoot)
}

func TestE2EMinimalImage(t *testing.T) {
	requireContainerRuntime(t)

	repoRoot := repoRoot(t)
	binDir := t.TempDir()
	nedPath := filepath.Join(binDir, "ned")
	buildBinary(t, repoRoot, "./cmd/ned", nedPath)

	imageTag := "docker.io/library/ned-minimal-e2e:arm64"
	t.Cleanup(func() {
		_, _ = runLoggedCommand(t, context.Background(), repoRoot, os.Environ(), "container", "image", "delete", imageTag)
		_, _ = runLoggedCommand(t, context.Background(), repoRoot, os.Environ(), "container", "builder", "delete", "--force")
	})
	buildCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if out, err := runLoggedCommand(t, buildCtx, repoRoot, os.Environ(), "container", "build", "-t", imageTag, filepath.Join("containers", "arm64", "minimal")); err != nil {
		if buildCtx.Err() == context.DeadlineExceeded {
			t.Fatal("container build minimal timed out")
		}
		t.Fatalf("container build minimal failed: %v\n%s", err, string(out))
	}

	// No agent CLI (or Node) is baked in; ned-agent-install fetches them on demand.
	t.Run("no-preinstalled-clis", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		out, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), "container", "run", "--rm", imageTag, "sh", "-c",
			"for c in claude codex gemini grok opencode node npm; do if command -v $c; then echo preinstalled=$c; exit 1; fi; done; "+
				"test -x /usr/local/bin/ned-agent-install && test -x /usr/local/bin/ned-agent-update && ned-agent-install --list")
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatal("preinstalled CLI check timed out")
		}
		if err != nil {
			t.Fatalf("image check failed: %v\n%s", err, string(out))
		}
		if !strings.Contains(string(out), "claude\ncodex\ngemini\ngrok\nopencode") {
			t.Fatalf("unexpected provider list:\n%s", string(out))
		}
	})

	t.Run("install-opencode", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		out, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), "container", "run", "--rm", imageTag, "sh", "-c",
			"ned-agent-install opencode && readlink /usr/local/bin/opencode && opencode --version")
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatal("opencode install timed out")
		}
		if err != nil {
			t.Fatalf("ned-agent-install opencode failed: %v\n%s", err, string(out))
		}
		if !strings.Contains(string(out), "/opt/ned-agents/.opencode/bin/opencode") || !regexp.MustCompile(`(?m)^\d+\.\d+\.\d+\s*$`).Match(out) {
			t.Fatalf("unexpected opencode install output:\n%s", string(out))
		}
	})

	t.Run("gentic", func(t *testing.T) {
		vmName := fmt.Sprintf("e2e-minimal-%d", time.Now().UnixNano())
		writeProfile(t, vmName, fmt.Sprintf("{\n  \"image\": %q,\n  \"flags\": { \"rm-container-on-exit\": true }\n}", imageTag))
		t.Cleanup(func() {
			cleanupVM(t, nedPath, repoRoot, vmName)
		})
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		args := []string{"gentic", vmName, vmName}
		cmd := exec.CommandContext(ctx, nedPath, args...)
		cmd.Dir = repoRoot
		cmd.Env = os.Environ()
		cmd.Stdin = strings.NewReader("exit\n")
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		t.Logf("CMD: %s", formatCommand(nedPath, args...))
		err := cmd.Run()
		logCommandOutput(t, output.Bytes())
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatal("ned gentic on minimal image timed out")
		}
		if err != nil {
			t.Fatalf("ned gentic on minimal image failed: %v\n%s", err, output.String())
		}
		hostUser, err := user.Current()
		if err != nil {
			t.Fatalf("current user: %v", err)
		}
		// The shell prompt names the attached user, proving the bootstrap hook
		// created the host-matching account instead of dropping into root.
		for _, want := range []string{"Bootstrap complete: ok", "Checking for agent CLI updates in the background", "Attached: profile=" + vmName, hostUser.Username + "@"} {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("expected %q in minimal gentic output:\n%s", want, output.String())
			}
		}
	})

	// Logins survive VM recreation on the auth volume; installed CLIs do not.
	// The boot updater reinstalls the CLI of every signed-in provider only.
	t.Run("update-reinstalls-signed-in", func(t *testing.T) {
		vmName := fmt.Sprintf("e2e-update-%d", time.Now().UnixNano())
		writeProfile(t, vmName, fmt.Sprintf("{\n  \"image\": %q\n}", imageTag))
		t.Cleanup(func() {
			cleanupVM(t, nedPath, repoRoot, vmName)
		})
		bootCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		boot := exec.CommandContext(bootCtx, nedPath, "gentic", vmName, vmName)
		boot.Dir = repoRoot
		boot.Env = os.Environ()
		boot.Stdin = strings.NewReader("exit\n")
		if out, err := boot.CombinedOutput(); err != nil {
			logCommandOutput(t, out)
			t.Fatalf("ned gentic for update check failed: %v", err)
		}
		hostUser, err := user.Current()
		if err != nil {
			t.Fatalf("current user: %v", err)
		}
		guestHome := nedruntime.GuestHomeForUser(hostUser.Username)

		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
		defer cancel()
		// A saved opencode login, as the auth volume keeps it across recreation.
		if out, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), "container", "exec", "--user", hostUser.Username, vmName, "env", "HOME="+guestHome, "sh", "-c",
			"test -O ~/.local && test -O ~/.local/share && mkdir -p ~/.local/share/opencode && printf '{}' > ~/.local/share/opencode/auth.json && readlink -f ~/.local/share/opencode"); err != nil || !strings.Contains(string(out), "/mnt/ned-auth/") {
			t.Fatalf("seed opencode login on the auth volume: %v\n%s", err, string(out))
		}
		out, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), "container", "exec", vmName, "env", "USER_NAME="+hostUser.Username, "ned-agent-update")
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatal("ned-agent-update timed out")
		}
		if err != nil {
			t.Fatalf("ned-agent-update failed: %v\n%s", err, string(out))
		}
		for _, want := range []string{"ok: install opencode (signed in)", "skip: claude (not installed, not signed in)", "skip: codex", "skip: gemini", "skip: grok", "opencode: "} {
			if !strings.Contains(string(out), want) {
				t.Fatalf("expected %q in ned-agent-update output:\n%s", want, string(out))
			}
		}
		if out, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), "container", "exec", "--user", hostUser.Username, vmName, "env", "HOME="+guestHome, "PATH=/usr/local/bin:/usr/bin:/bin", "sh", "-c",
			"opencode --version && ! command -v claude"); err != nil {
			t.Fatalf("opencode not usable by the VM user after the update: %v\n%s", err, string(out))
		}
		// The VM user installs through passwordless sudo, as ned-agent-login does.
		if out, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), "container", "exec", "--user", hostUser.Username, vmName, "env", "HOME="+guestHome, "PATH=/usr/local/bin:/usr/bin:/bin", "sh", "-c",
			"ned-agent-install grok && grok --version && test ! -e ~/.grok/bin"); err != nil || !strings.Contains(string(out), "grok ") {
			t.Fatalf("VM user install of grok failed: %v\n%s", err, string(out))
		}
	})

	t.Run("desktop-tools", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		out, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), "container", "run", "--rm", imageTag,
			"sh", "-c", "for tool in chromium ned-desktop xterm jwm Xvfb x11vnc websockify; do command -v $tool || exit 1; done; test -f /usr/share/novnc/vnc.html && chromium --version")
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatal("desktop tools check timed out")
		}
		if err != nil {
			t.Fatalf("desktop tools missing: %v\n%s", err, string(out))
		}
		for _, want := range []string{"/usr/bin/chromium", "/usr/local/bin/ned-desktop", "/usr/bin/xterm", "Chromium "} {
			if !strings.Contains(string(out), want) {
				t.Fatalf("expected %q in desktop tools output:\n%s", want, string(out))
			}
		}
	})

	t.Run("desktop", func(t *testing.T) {
		vmName := fmt.Sprintf("e2e-desktop-%d", time.Now().UnixNano())
		writeProfile(t, vmName, fmt.Sprintf("{\n  \"image\": %q\n}", imageTag))
		t.Cleanup(func() {
			cleanupVM(t, nedPath, repoRoot, vmName)
		})

		// Boot the VM and detach; it keeps running for the desktop.
		bootCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		boot := exec.CommandContext(bootCtx, nedPath, "gentic", vmName, vmName)
		boot.Dir = repoRoot
		boot.Env = os.Environ()
		boot.Stdin = strings.NewReader("exit\n")
		if out, err := boot.CombinedOutput(); err != nil {
			logCommandOutput(t, out)
			t.Fatalf("ned gentic for desktop failed: %v", err)
		}

		// Stub macOS open so the test captures the URL instead of launching a browser.
		stubDir := t.TempDir()
		urlFile := filepath.Join(stubDir, "url")
		stub := "#!/bin/sh\nprintf '%s' \"$1\" > " + strconv.Quote(urlFile) + "\n"
		if err := os.WriteFile(filepath.Join(stubDir, "open"), []byte(stub), 0700); err != nil {
			t.Fatalf("write open stub: %v", err)
		}
		env := append(os.Environ(), "PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		out, err := runLoggedCommand(t, ctx, repoRoot, env, nedPath, "desktop", vmName)
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatal("ned desktop timed out")
		}
		if err != nil {
			t.Fatalf("ned desktop failed: %v\n%s", err, string(out))
		}
		if !strings.Contains(string(out), "Desktop: http://") || strings.Contains(string(out), "password=") {
			t.Fatalf("expected a Desktop URL without the password:\n%s", string(out))
		}
		rawURL, err := os.ReadFile(urlFile)
		if err != nil {
			t.Fatalf("open stub was not called: %v", err)
		}
		pageURL, err := url.Parse(string(rawURL))
		if err != nil {
			t.Fatalf("parse desktop URL: %v", err)
		}
		if pageURL.Port() != "6080" || pageURL.Path != "/vnc.html" || len(pageURL.Query().Get("password")) != 8 {
			t.Fatalf("unexpected desktop URL shape: %s://%s%s", pageURL.Scheme, pageURL.Host, pageURL.Path)
		}

		// noVNC answers inside the VM and from the host on the VM's IP.
		inVM, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), "container", "exec", vmName,
			"bash", "-c", "exec 3<>/dev/tcp/127.0.0.1/6080 && echo port-6080-open")
		if err != nil || !strings.Contains(string(inVM), "port-6080-open") {
			t.Fatalf("port 6080 not open inside the VM: %v\n%s", err, string(inVM))
		}
		// Use the system curl: macOS Local Network privacy blocks freshly built
		// test binaries from dialing VM addresses ("no route to host").
		pageOnly := pageURL.Scheme + "://" + pageURL.Host + pageURL.Path
		status, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), "/usr/bin/curl", "-sS", "--max-time", "10", "-o", "/dev/null", "-w", "%{http_code}", pageOnly)
		if err != nil || strings.TrimSpace(string(status)) != "200" {
			t.Fatalf("host cannot load %s: %v (%s)", pageOnly, err, string(status))
		}

		// x11vnc and Chromium DevTools stay on VM loopback.
		for _, port := range []string{"5901", "9222"} {
			probe := "http://" + net.JoinHostPort(pageURL.Hostname(), port) + "/"
			if _, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), "/usr/bin/curl", "-sS", "--max-time", "5", "-o", "/dev/null", probe); err == nil {
				t.Fatalf("port %s is reachable from the host; it must stay on VM loopback", port)
			}
		}

		if out, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), nedPath, "desktop", "stop", vmName); err != nil {
			t.Fatalf("ned desktop stop failed: %v\n%s", err, string(out))
		}
	})
}

func TestE2EProfileEnvRejected(t *testing.T) {
	requireContainerRuntime(t)

	repoRoot := repoRoot(t)
	binDir := t.TempDir()
	nedPath := filepath.Join(binDir, "ned")
	buildBinary(t, repoRoot, "./cmd/ned", nedPath)

	vmName := fmt.Sprintf("e2e-env-%d", time.Now().UnixNano())
	writeProfile(t, vmName, "{\"env\": {\"FOO\": \"bar\"}}")
	t.Cleanup(func() {
		cleanupVM(t, nedPath, repoRoot, vmName)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, nedPath, "gentic", vmName, vmName)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	t.Logf("CMD: %s", formatCommand(nedPath, "gentic", vmName, vmName))
	err := cmd.Run()
	logCommandOutput(t, output.Bytes())
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("ned gentic env rejection timed out")
	}
	if err == nil {
		t.Fatalf("expected env schema error, got success\n%s", output.String())
	}
	if !strings.Contains(output.String(), `unknown field "env"`) {
		t.Fatalf("expected env schema error, got:\n%s", output.String())
	}
}

// writeProfile writes a throwaway profile under ~/.ned/profiles/personal and
// removes it when the test ends.
func writeProfile(t *testing.T, name, config string) {
	t.Helper()
	home, err := nedprofile.ResolveHome()
	if err != nil {
		t.Fatalf("resolve home: %v", err)
	}
	configPath := nedprofile.ProfileConfigPath(home, name)
	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); err == nil {
		t.Fatalf("profile %s already exists", dir)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("create profile dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
}

func requireContainerRuntime(t *testing.T) {
	t.Helper()
	containerPath, err := exec.LookPath("container")
	if err != nil {
		t.Skipf("container runtime not found: %v", err)
	}
	if err := nedruntime.RequireContainerRuntimeVersion(containerPath); err != nil {
		t.Skipf("container runtime unsupported for E2E: %v", err)
	}
}

func buildBinary(t *testing.T, repoRoot, pkg, outPath string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	out, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), "go", "build", "-o", outPath, pkg)
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("go build timed out: %s", pkg)
	}
	if err != nil {
		t.Fatalf("failed to build %s: %v\n%s", pkg, err, string(out))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	return filepath.Dir(cwd)
}

func logVMs(t *testing.T, nedPath, repoRoot string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := runLoggedCommand(t, ctx, repoRoot, os.Environ(), nedPath, "list")
	if err != nil {
		t.Logf("ned list failed: %v", err)
	}
}

func cleanupVM(t *testing.T, nedPath, repoRoot, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, nedPath, "remove", name)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	_ = cmd.Run()
	// ned keeps a VM's staging directory; drop the one this test created.
	if home, err := nedprofile.ResolveHome(); err == nil {
		if dir, err := nedprofile.StagingVMDir(home, name); err == nil {
			_ = os.RemoveAll(dir)
		}
	}
}

func runLoggedCommand(t *testing.T, ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	t.Helper()
	t.Logf("CMD: %s", formatCommand(name, args...))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	logCommandOutput(t, out)
	return out, err
}

func formatCommand(name string, args ...string) string {
	if len(args) == 0 {
		return name
	}
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, name)
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\n\"") {
			parts = append(parts, strconv.Quote(arg))
		} else {
			parts = append(parts, arg)
		}
	}
	return strings.Join(parts, " ")
}

func logCommandOutput(t *testing.T, out []byte) {
	t.Helper()
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return
	}
	t.Logf("OUTPUT:\n%s", trimmed)
}
