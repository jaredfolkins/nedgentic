package e2e

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestE2ERuntimeCommandsDryRunBrewPaths(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("runtime CLI brew-path e2e only runs on macOS")
	}

	repo := repoRoot(t)
	binDir := t.TempDir()
	nedPath := filepath.Join(binDir, "ned")
	buildBinary(t, repo, "./cmd/ned", nedPath)

	brewOnlyDir := t.TempDir()
	brewOnlyPath := writeExecutableScript(t, brewOnlyDir, "brew", fakeBrewScript())
	envBrewOnly := withEnvOverrides(os.Environ(), map[string]string{
		"PATH": brewOnlyDir,
	})

	installOut := runRuntimeCommand(t, repo, envBrewOnly, nedPath, "runtime", "install", "--dry-run")
	requireOutputContains(t, installOut, "Homebrew detected at "+brewOnlyPath)
	requireOutputContains(t, installOut, "DRY RUN: brew install container")
	requireOutputContains(t, installOut, "DRY RUN: container system start --enable-kernel-install")

	brewContainerDir := t.TempDir()
	containerPath := writeExecutableScript(t, brewContainerDir, "container", fakeContainerScript())
	brewPath := writeExecutableScript(t, brewContainerDir, "brew", fakeBrewScript())
	envBrewContainer := withEnvOverrides(os.Environ(), map[string]string{
		"PATH": brewContainerDir,
	})

	statusOut := runRuntimeCommand(t, repo, envBrewContainer, nedPath, "runtime", "status")
	requireOutputContains(t, statusOut, "Runtime status:")
	requireOutputContains(t, statusOut, "container: installed")
	requireOutputContains(t, statusOut, "container path: "+containerPath)
	requireOutputContains(t, statusOut, "homebrew: installed")
	requireOutputContains(t, statusOut, "homebrew path: "+brewPath)
	requireOutputContains(t, statusOut, "homebrew container: container 1.6.0")

	restartOut := runRuntimeCommand(t, repo, envBrewContainer, nedPath, "runtime", "restart", "--dry-run")
	requireOutputContains(t, restartOut, "DRY RUN: container system stop")
	requireOutputContains(t, restartOut, "DRY RUN: container system start --enable-kernel-install")

	removeOut := runRuntimeCommand(t, repo, envBrewContainer, nedPath, "runtime", "remove", "--dry-run")
	requireOutputContains(t, removeOut, "Removing container via Homebrew...")
	requireOutputContains(t, removeOut, "DRY RUN: "+containerPath+" system stop")
	requireOutputContains(t, removeOut, "DRY RUN: "+brewPath+" uninstall container")
}

func TestE2ERuntimeCommandsDryRunNonBrewPaths(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("runtime CLI non-brew-path e2e only runs on macOS")
	}

	repo := repoRoot(t)
	binDir := t.TempDir()
	nedPath := filepath.Join(binDir, "ned")
	buildBinary(t, repo, "./cmd/ned", nedPath)

	fakeBin := t.TempDir()
	containerPath := writeExecutableScript(t, fakeBin, "container", fakeContainerScript())
	env := withEnvOverrides(os.Environ(), map[string]string{
		"PATH":                              fakeBin,
		"NED_TEST_INSTALLER_SUDO":           "doas",
		"NED_TEST_INSTALLER_UNINSTALL_PATH": "/tmp/fake-uninstall.sh",
	})

	statusOut := runRuntimeCommand(t, repo, env, nedPath, "runtime", "status")
	requireOutputContains(t, statusOut, "container: installed")
	requireOutputContains(t, statusOut, "container path: "+containerPath)
	requireOutputContains(t, statusOut, "homebrew: not found")

	installOut := runRuntimeCommand(t, repo, env, nedPath, "runtime", "install", "--dry-run")
	requireOutputContains(t, installOut, "container runtime already installed at "+containerPath)
	requireOutputContains(t, installOut, "DRY RUN: "+containerPath+" system start --enable-kernel-install")

	removeOut := runRuntimeCommand(t, repo, env, nedPath, "runtime", "remove", "--dry-run")
	requireOutputContains(t, removeOut, "Homebrew not found; removing runtime with signed pkg uninstall.")
	requireOutputContains(t, removeOut, "DRY RUN: "+containerPath+" system stop")
	requireOutputContains(t, removeOut, "DRY RUN: doas /tmp/fake-uninstall.sh -d")

	restartOut := runRuntimeCommand(t, repo, env, nedPath, "runtime", "restart", "--dry-run")
	requireOutputContains(t, restartOut, "DRY RUN: container system stop")
	requireOutputContains(t, restartOut, "DRY RUN: container system start --enable-kernel-install")
}

func TestE2ENetworkAndRuntimeDNSCommandAssembly(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("network/dns command assembly e2e only runs on macOS")
	}

	repo := repoRoot(t)
	binDir := t.TempDir()
	nedPath := filepath.Join(binDir, "ned")
	buildBinary(t, repo, "./cmd/ned", nedPath)

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "container-args.log")
	containerPath := writeExecutableScript(t, fakeBin, "container", fakeContainerScriptWithArgLog())

	env := withEnvOverrides(os.Environ(), map[string]string{
		"PATH":                       fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"NED_E2E_CONTAINER_ARGS_LOG": logPath,
	})

	networkOut := runRuntimeCommand(t, repo, env, nedPath,
		"--container-bin", containerPath,
		"network", "create", "--internal", "team-internal",
	)
	requireOutputContains(t, networkOut, "network created")

	networkRemoveOut := runRuntimeCommand(t, repo, env, nedPath,
		"--container-bin", containerPath,
		"network", "remove", "team-internal",
	)
	requireOutputContains(t, networkRemoveOut, "network deleted")

	dnsListOut := runRuntimeCommand(t, repo, env, nedPath,
		"--container-bin", containerPath,
		"runtime", "dns", "list", "--format", "json", "--quiet",
	)
	requireOutputContains(t, dnsListOut, "dev.test")

	dnsCreateOut := runRuntimeCommand(t, repo, env, nedPath,
		"--container-bin", containerPath,
		"runtime", "dns", "create", "dev.test", "--localhost", "host.gateway",
	)
	requireOutputContains(t, dnsCreateOut, "dns created")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read container arg log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	requireLoggedCommandContains(t, lines, "network create --internal team-internal")
	requireLoggedCommandContains(t, lines, "network delete team-internal")
	requireLoggedCommandContains(t, lines, "system dns list --format json --quiet")
	requireLoggedCommandContains(t, lines, "system dns create --localhost host.gateway dev.test")
}

func TestE2ERuntimeExportAndStatusFormatAssembly(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("runtime/export command assembly e2e only runs on macOS")
	}

	repo := repoRoot(t)
	binDir := t.TempDir()
	nedPath := filepath.Join(binDir, "ned")
	buildBinary(t, repo, "./cmd/ned", nedPath)

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "container-args.log")
	containerPath := writeExecutableScript(t, fakeBin, "container", fakeContainerScriptWithArgLog())

	env := withEnvOverrides(os.Environ(), map[string]string{
		"PATH":                       fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"NED_E2E_CONTAINER_ARGS_LOG": logPath,
	})

	exportOut := runRuntimeCommand(t, repo, env, nedPath,
		"--container-bin", containerPath,
		"export", "sandbox-vm", "--output", "/tmp/sandbox.tar",
	)
	requireOutputContains(t, exportOut, "Export: sandbox-vm... done")

	statusOut := runRuntimeCommand(t, repo, env, nedPath,
		"--container-bin", containerPath,
		"runtime", "status", "--format", "json",
	)
	requireOutputContains(t, statusOut, `{"status":"running"}`)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read container arg log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	requireLoggedCommandContains(t, lines, "export --output /tmp/sandbox.tar sandbox-vm")
	requireLoggedCommandContains(t, lines, "system status --format json")
}

func TestE2ENetworkAndRuntimeDNSFailureMessaging(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("network/dns failure-message e2e only runs on macOS")
	}

	repo := repoRoot(t)
	binDir := t.TempDir()
	nedPath := filepath.Join(binDir, "ned")
	buildBinary(t, repo, "./cmd/ned", nedPath)

	fakeBin := t.TempDir()
	containerPath := writeExecutableScript(t, fakeBin, "container", fakeContainerScriptWithNetworkAndDNSFailures())

	env := withEnvOverrides(os.Environ(), map[string]string{
		"PATH": fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
	})

	networkFailOut := runRuntimeCommandExpectError(t, repo, env, nedPath,
		"--container-bin", containerPath,
		"network", "create", "--internal", "team-internal",
	)
	requireOutputContains(t, networkFailOut, "Network create failed:")
	requireOutputContains(t, networkFailOut, "network denied")

	dnsFailOut := runRuntimeCommandExpectError(t, repo, env, nedPath,
		"--container-bin", containerPath,
		"runtime", "dns", "create", "dev.test", "--localhost", "host.gateway",
	)
	requireOutputContains(t, dnsFailOut, "Runtime DNS create failed:")
	requireOutputContains(t, dnsFailOut, "dns denied")
}

func runRuntimeCommand(t *testing.T, repo string, env []string, nedPath string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := runLoggedCommand(t, ctx, repo, env, nedPath, args...)
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("command timed out: %s", formatCommand(nedPath, args...))
	}
	if err != nil {
		t.Fatalf("command failed: %s: %v\n%s", formatCommand(nedPath, args...), err, string(out))
	}
	return stripANSI(string(out))
}

func runRuntimeCommandExpectError(t *testing.T, repo string, env []string, nedPath string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := runLoggedCommand(t, ctx, repo, env, nedPath, args...)
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("command timed out: %s", formatCommand(nedPath, args...))
	}
	if err == nil {
		t.Fatalf("expected command failure: %s\n%s", formatCommand(nedPath, args...), string(out))
	}
	return stripANSI(string(out))
}

func requireOutputContains(t *testing.T, output, expected string) {
	t.Helper()
	if !strings.Contains(output, expected) {
		t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
	}
}

func writeExecutableScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0700); err != nil {
		t.Fatalf("write fake executable %s: %v", name, err)
	}
	return path
}

func withEnvOverrides(base []string, overrides map[string]string) []string {
	out := make([]string, 0, len(base)+len(overrides))
	for _, item := range base {
		key := item
		if before, _, ok := strings.Cut(item, "="); ok {
			key = before
		}
		if _, ok := overrides[key]; ok {
			continue
		}
		out = append(out, item)
	}
	for key, value := range overrides {
		out = append(out, key+"="+value)
	}
	return out
}

func fakeBrewScript() string {
	return "#!/bin/sh\n" +
		"if [ \"$1\" = \"list\" ] && [ \"$2\" = \"--versions\" ] && [ \"$3\" = \"container\" ]; then\n" +
		"  echo \"container 1.6.0\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
}

func fakeContainerScript() string {
	return "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ] || [ \"$1\" = \"version\" ]; then\n" +
		"  echo \"container 1.6.0\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
}

func fakeContainerScriptWithArgLog() string {
	return "#!/bin/sh\n" +
		"if [ -n \"$NED_E2E_CONTAINER_ARGS_LOG\" ]; then\n" +
		"  echo \"$*\" >> \"$NED_E2E_CONTAINER_ARGS_LOG\"\n" +
		"fi\n" +
		"if [ \"$1\" = \"--version\" ] || [ \"$1\" = \"version\" ]; then\n" +
		"  echo \"container version 1.6.0\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"network\" ] && [ \"$2\" = \"create\" ]; then\n" +
		"  echo \"network created\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"network\" ] && [ \"$2\" = \"delete\" ]; then\n" +
		"  echo \"network deleted\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"system\" ] && [ \"$2\" = \"dns\" ] && [ \"$3\" = \"list\" ]; then\n" +
		"  echo \"dev.test\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"system\" ] && [ \"$2\" = \"dns\" ] && [ \"$3\" = \"create\" ]; then\n" +
		"  echo \"dns created\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"export\" ]; then\n" +
		"  echo \"exported\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"system\" ] && [ \"$2\" = \"status\" ] && [ \"$3\" = \"--format\" ] && [ \"$4\" = \"json\" ]; then\n" +
		"  echo '{\"status\":\"running\"}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
}

func fakeContainerScriptWithNetworkAndDNSFailures() string {
	return "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ] || [ \"$1\" = \"version\" ]; then\n" +
		"  echo \"container version 1.6.0\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"network\" ] && [ \"$2\" = \"create\" ]; then\n" +
		"  echo \"network denied\" >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"if [ \"$1\" = \"system\" ] && [ \"$2\" = \"dns\" ] && [ \"$3\" = \"create\" ]; then\n" +
		"  echo \"dns denied\" >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"exit 0\n"
}

func requireLoggedCommandContains(t *testing.T, lines []string, want string) {
	t.Helper()
	for _, line := range lines {
		if strings.TrimSpace(line) == strings.TrimSpace(want) {
			return
		}
	}
	t.Fatalf("expected command log to contain %q, got:\n%s", want, strings.Join(lines, "\n"))
}
