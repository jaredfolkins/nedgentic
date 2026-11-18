package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"nedgentic/internal/release"
	"nedgentic/internal/scripts"
)

func dockerfileImageDirsForArch(t *testing.T, archDir string) []string {
	t.Helper()

	root := filepath.Join("..", "..", "containers", archDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s image directory: %v", archDir, err)
	}

	images := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, entry.Name(), "Dockerfile")); err == nil {
			images = append(images, entry.Name())
		}
	}
	sort.Strings(images)
	return images
}

func containerDirsForArch(t *testing.T, archDir string) []string {
	t.Helper()

	root := filepath.Join("..", "..", "containers", archDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s container directory: %v", archDir, err)
	}

	dirs := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

func readContainerFile(t *testing.T, parts ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{"..", "..", "containers"}, parts...)...)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func requireContains(t *testing.T, label, text string, needles ...string) {
	t.Helper()

	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			t.Fatalf("expected %s to include %q", label, needle)
		}
	}
}

func requireOmits(t *testing.T, label, text string, needles ...string) {
	t.Helper()

	for _, needle := range needles {
		if strings.Contains(text, needle) {
			t.Fatalf("expected %s to avoid %q", label, needle)
		}
	}
}

func TestContainerImageDirectoriesAreMinimalOnly(t *testing.T) {
	// ned is arm64-only: no x86 image and no bulk build/push scripts.
	if got := containerDirsForArch(t, "."); strings.Join(got, ",") != "arm64" {
		t.Fatalf("expected only containers/arm64, got %v", got)
	}
	for _, removed := range []string{"x86", "make_all.sh", "push_all.sh"} {
		path := filepath.Join("..", "..", "containers", removed)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s should not exist (err=%v)", path, err)
		}
	}
	if got := dockerfileImageDirsForArch(t, "arm64"); strings.Join(got, ",") != "minimal" {
		t.Fatalf("expected only the minimal image dir, got %v", got)
	}
	for _, dir := range containerDirsForArch(t, "arm64") {
		if dir != "minimal" && dir != "dist" {
			t.Fatalf("unexpected directory containers/arm64/%s", dir)
		}
	}
}

func TestDeletedImageTemplateInitializerRemoved(t *testing.T) {
	path := filepath.Join("..", "..", "scripts", "init-image-templates.sh")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s should not exist; default image hooks were deleted with the old image contexts", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func TestMinimalDockerfileIsLeanWithoutAgentCLIs(t *testing.T) {
	label := "arm64/minimal/Dockerfile"
	text := readContainerFile(t, "arm64", "minimal", "Dockerfile")
	requireContains(t, label, text,
		"FROM --platform=linux/arm64 docker.io/library/debian:trixie-slim",
		"--no-install-recommends",
		"'path-exclude=/usr/share/doc/*'",
		"'path-include=/usr/share/doc/*/copyright'",
		"'path-exclude=/usr/share/man/*'",
		"'path-exclude=/usr/share/locale/*'",
		"rm -rf /var/lib/apt/lists/*",
		"    curl \\\n",
		"    git \\\n",
		"    sudo \\\n",
		"    chromium \\\n",
		"    fonts-dejavu-core \\\n",
		"    jwm \\\n",
		"    python3-websockify \\\n",
		"    x11vnc \\\n",
		"    xterm \\\n",
		"    xvfb; \\\n",
		"apt-get download novnc",
		"COPY --chmod=755 scripts/ned-agent-install scripts/ned-agent-update scripts/ned-desktop /usr/local/bin/",
		"COPY --chmod=644 scripts/ned-desktop-jwmrc /etc/ned-desktop/jwmrc",
		"COPY --chmod=755 hooks/*.sh /ned/hooks/",
	)
	// Agent CLIs and Node install on demand (ned-agent-install), never at build.
	requireOmits(t, label, text,
		"claude.ai", "downloads.claude.ai", "openai/codex", "x.ai/cli", "opencode.ai",
		"gemini-cli", "opencode-ai", "nodejs", "npm", "CLI_VERSION", "_SHA256",
		"    novnc \\\n", "jq", "ripgrep",
		"kali", "mitmproxy", "ned-observe", "FIRECRACKER", "apt-get upgrade",
	)
	if strings.Count(text, "\nRUN ") != 1 {
		t.Fatalf("expected a single RUN layer in %s", label)
	}

	makefile := readContainerFile(t, "arm64", "minimal", "Makefile")
	requireContains(t, "arm64/minimal/Makefile", makefile,
		"IMAGE_NAME := ned-minimal",
		"TAG ?= dev",
		"container build $(BUILD_FLAGS) -t $(IMAGE_NAME):$(TAG) .",
		"update: BUILD_FLAGS += --pull",
	)
	requireOmits(t, "arm64/minimal/Makefile", makefile, "jfolkins", "push", "ned storage", "ned image")
	if !strings.HasSuffix(release.DevImageRef, ":dev") || !strings.HasPrefix(release.DevImageRef, "ned-minimal:") {
		t.Fatalf("release.DevImageRef %q must match the Makefile default tag", release.DevImageRef)
	}

	hook := readContainerFile(t, "arm64", "minimal", "hooks", "03-agent-updates.sh")
	requireContains(t, "arm64/minimal/hooks/03-agent-updates.sh", hook,
		"mkdir \"$LOCK_DIR\"",
		"nohup setsid /usr/local/bin/ned-agent-update",
		"</dev/null &",
	)
}

func TestMinimalAgentInstallerUsesOfficialInstallers(t *testing.T) {
	label := "arm64/minimal/scripts/ned-agent-install"
	text := readContainerFile(t, "arm64", "minimal", "scripts", "ned-agent-install")
	requireContains(t, label, text,
		`PROVIDERS="claude codex gemini grok opencode"`,
		"AGENT_HOME=/opt/ned-agents",
		"fetch_installer https://claude.ai/install.sh",
		`agent_env CLAUDE_INSTALL_ALLOW_SUDO=1 bash "$installer"`,
		"fetch_installer https://github.com/openai/codex/releases/latest/download/install.sh",
		`agent_env CODEX_HOME="$AGENT_HOME/.codex" CODEX_NON_INTERACTIVE=1 sh "$installer"`,
		"fetch_installer https://x.ai/cli/install.sh",
		"fetch_installer https://opencode.ai/install",
		`agent_env bash "$installer" --no-modify-path`,
		"env -u CLAUDE_CONFIG_DIR -u CODEX_HOME",
		"https://nodejs.org/dist/index.json",
		"sha256sum -c -",
		"@google/gemini-cli@latest",
		`flock -w "$LOCK_TIMEOUT" 9`,
		`exec sudo -n --preserve-env="$keep" "$0"`,
	)
	// Latest releases only: nothing pinned.
	requireOmits(t, label, text, "_VERSION=", "CODEX_RELEASE=", "--version ")

	updater := readContainerFile(t, "arm64", "minimal", "scripts", "ned-agent-update")
	requireContains(t, "arm64/minimal/scripts/ned-agent-update", updater,
		"ned-agent-install --list",
		`step "update $id" ned-agent-install --update "$id"`,
		`ned-agent-install --signed-in "$id" "$HOME_DIR"`,
		`step "install $id (signed in)" ned-agent-install "$id"`,
		"READY_FILE=/tmp/ned-ready",
		`timeout "$STEP_TIMEOUT"`,
	)
	requireOmits(t, "arm64/minimal/scripts/ned-agent-update", updater, "npm-global", "claude update", "@latest")

	for _, script := range []string{
		filepath.Join("hooks", "01-bootstrap.sh"),
		filepath.Join("hooks", "03-agent-updates.sh"),
		filepath.Join("scripts", "ned-agent-install"),
		filepath.Join("scripts", "ned-agent-update"),
	} {
		path := filepath.Join("..", "..", "containers", "arm64", "minimal", script)
		if out, err := exec.Command("sh", "-n", path).CombinedOutput(); err != nil {
			t.Fatalf("sh -n %s: %v\n%s", path, err, out)
		}
	}
}

// ned-agent-install --signed-in and ned-agent-login must agree on providers
// and on the credential file that marks each one signed in.
func TestAgentInstallerMatchesLoginCredentials(t *testing.T) {
	installer := filepath.Join("..", "..", "containers", "arm64", "minimal", "scripts", "ned-agent-install")
	out, err := exec.Command("sh", installer, "--list").Output()
	if err != nil {
		t.Fatalf("--list: %v", err)
	}
	ids := strings.Fields(string(out))
	login := scripts.AgentLoginScript()
	entry := regexp.MustCompile(`"([a-z]+)\|[^|]*\|\$HOME/([^|]+)\|`)
	loginCreds := map[string]string{}
	for _, match := range entry.FindAllStringSubmatch(login, -1) {
		loginCreds[match[1]] = match[2]
	}
	if len(ids) != 5 || len(loginCreds) != len(ids) {
		t.Fatalf("providers differ: installer %v, login %v", ids, loginCreds)
	}
	for _, id := range ids {
		cred, ok := loginCreds[id]
		if !ok {
			t.Fatalf("ned-agent-login has no %s entry", id)
		}
		home := t.TempDir()
		signedIn := func() bool {
			return exec.Command("sh", installer, "--signed-in", id, home).Run() == nil
		}
		if signedIn() {
			t.Fatalf("%s reported signed in with an empty home", id)
		}
		path := filepath.Join(home, cred)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
		if !signedIn() {
			t.Fatalf("%s not signed in with %s present", id, cred)
		}
	}
	if exec.Command("sh", installer, "--signed-in", "bogus", t.TempDir()).Run() == nil {
		t.Fatalf("unknown provider must fail")
	}
}

func TestMinimalDesktopScripts(t *testing.T) {
	desktop := readContainerFile(t, "arm64", "minimal", "scripts", "ned-desktop")
	requireContains(t, "arm64/minimal/scripts/ned-desktop", desktop,
		`die "run as the VM user, not root"`,
		`-nolisten tcp`,
		`x11vnc -display "$DISPLAY" -localhost -rfbport "$VNC_PORT"`,
		`-passwdfile "$PASSWORD_FILE"`,
		`websockify --web "$NOVNC_WEB" "0.0.0.0:$NOVNC_PORT" "127.0.0.1:$VNC_PORT"`,
		`--remote-debugging-address=127.0.0.1`,
		`exec xterm`,
		`(umask 077 && printf '%s\n' "$password" >"$PASSWORD_FILE")`,
		`echo "NED_DESKTOP port=$NOVNC_PORT password=$(cat "$PASSWORD_FILE")"`,
	)
	// Chromium keeps its sandbox; the password stays out of argv and logs.
	requireOmits(t, "arm64/minimal/scripts/ned-desktop", desktop,
		"--no-sandbox",
		"-passwd \"$password\"",
		"-storepasswd",
	)
	jwmrc := readContainerFile(t, "arm64", "minimal", "scripts", "ned-desktop-jwmrc")
	requireContains(t, "arm64/minimal/scripts/ned-desktop-jwmrc", jwmrc,
		`<Program label="Terminal">ned-desktop terminal</Program>`,
		`<Key mask="CA" key="t">exec:ned-desktop terminal</Key>`,
	)
	if bash, err := exec.LookPath("bash"); err == nil {
		path := filepath.Join("..", "..", "containers", "arm64", "minimal", "scripts", "ned-desktop")
		if out, err := exec.Command(bash, "-n", path).CombinedOutput(); err != nil {
			t.Fatalf("bash -n %s: %v\n%s", path, err, out)
		}
	}
}

func TestMinimalBootstrapHookKeepsAgentPath(t *testing.T) {
	text := readContainerFile(t, "arm64", "minimal", "hooks", "01-bootstrap.sh")
	requireContains(t, "arm64/minimal/hooks/01-bootstrap.sh", text,
		"export PATH=/usr/local/bin:$PATH # ned-agent-path",
		`echo "$USER_NAME ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/90-ned-user`,
	)
	requireOmits(t, "arm64/minimal/hooks/01-bootstrap.sh", text, "npm-global")
}
