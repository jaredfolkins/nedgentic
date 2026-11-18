package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestE2EStorageSummaryIncludesOtherRuntimeDataNote(t *testing.T) {
	requireContainerRuntime(t)

	repo := repoRoot(t)
	binDir := t.TempDir()
	nedPath := filepath.Join(binDir, "ned")
	buildBinary(t, repo, "./cmd/ned", nedPath)

	fakeBinDir := t.TempDir()
	dataRoot := filepath.Join(t.TempDir(), "runtime-root")
	if err := os.MkdirAll(dataRoot, 0700); err != nil {
		t.Fatalf("mkdir data root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataRoot, "state.bin"), []byte(strings.Repeat("a", 2048)), 0600); err != nil {
		t.Fatalf("write data root payload: %v", err)
	}
	fakeContainerPath := writeExecutableScript(t, fakeBinDir, "container", fakeStorageContainerScript(dataRoot))

	env := withEnvOverrides(os.Environ(), map[string]string{
		"PATH": fakeBinDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	})

	output := runNedCommand(t, repo, env, nedPath,
		"--container-bin", fakeContainerPath,
		"storage", "list", "--all",
	)
	requireOutputContains(t, output, "Storage summary:")
	requireOutputContains(t, output, "Runtime data root: "+dataRoot)
	requireOutputContains(t, output, "Other runtime data (build cache, logs, rootfs):")
}

func runNedCommand(t *testing.T, repo string, env []string, nedPath string, args ...string) string {
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

func fakeStorageContainerScript(dataRoot string) string {
	return "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ] || [ \"$1\" = \"version\" ]; then\n" +
		"  echo \"container 1.6.0\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"volume\" ] && [ \"$2\" = \"list\" ] && [ \"$3\" = \"--format\" ] && [ \"$4\" = \"json\" ]; then\n" +
		"  echo \"[]\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"system\" ] && [ \"$2\" = \"df\" ] && [ \"$3\" = \"--format\" ] && [ \"$4\" = \"json\" ]; then\n" +
		"  echo '{\"containers\":{\"active\":0,\"total\":1,\"sizeInBytes\":1024,\"reclaimable\":512},\"images\":{\"active\":1,\"total\":2,\"sizeInBytes\":4096,\"reclaimable\":1024},\"volumes\":{\"active\":0,\"total\":0,\"sizeInBytes\":0,\"reclaimable\":0}}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"system\" ] && [ \"$2\" = \"status\" ] && [ \"$3\" = \"--format\" ] && [ \"$4\" = \"json\" ]; then\n" +
		"  echo '{\"status\":\"running\",\"paths\":{\"appRoot\":" + fmt.Sprintf("%q", dataRoot) + "}}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
}
