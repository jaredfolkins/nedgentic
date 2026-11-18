package runtime

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestParseLatestReleaseAsset(t *testing.T) {
	body := []byte(`{"tag_name":"0.8.0","assets":[{"name":"notes.txt","browser_download_url":"https://example/notes.txt"},{"name":"` + ContainerSignedPkgName + `","browser_download_url":"https://example/` + ContainerSignedPkgName + `"}]}`)
	asset, err := parseLatestReleaseAsset(body)
	if err != nil {
		t.Fatalf("parseLatestReleaseAsset error: %v", err)
	}
	if asset.Tag != "0.8.0" {
		t.Fatalf("unexpected tag: %s", asset.Tag)
	}
	if asset.Name != ContainerSignedPkgName {
		t.Fatalf("unexpected asset name: %s", asset.Name)
	}
	if asset.URL != "https://example/"+ContainerSignedPkgName {
		t.Fatalf("unexpected asset URL: %s", asset.URL)
	}
}

func TestParseLatestReleaseAssetAcceptsVersionedSignedPkg(t *testing.T) {
	const versionedName = "container-0.10.0-installer-signed.pkg"
	body := []byte(`{"tag_name":"0.10.0","assets":[{"name":"container-installer-unsigned.pkg","browser_download_url":"https://example/container-installer-unsigned.pkg"},{"name":"` + versionedName + `","browser_download_url":"https://example/` + versionedName + `"}]}`)
	asset, err := parseLatestReleaseAsset(body)
	if err != nil {
		t.Fatalf("parseLatestReleaseAsset error: %v", err)
	}
	if asset.Tag != "0.10.0" {
		t.Fatalf("unexpected tag: %s", asset.Tag)
	}
	if asset.Name != versionedName {
		t.Fatalf("unexpected asset name: %s", asset.Name)
	}
	if asset.URL != "https://example/"+versionedName {
		t.Fatalf("unexpected asset URL: %s", asset.URL)
	}
}

func TestParseLatestReleaseAssetRejectsMultipleVersionedSignedPkgs(t *testing.T) {
	body := []byte(`{"tag_name":"0.10.0","assets":[{"name":"container-0.10.0-installer-signed.pkg","browser_download_url":"https://example/container-0.10.0-installer-signed.pkg"},{"name":"container-0.10.1-installer-signed.pkg","browser_download_url":"https://example/container-0.10.1-installer-signed.pkg"}]}`)
	_, err := parseLatestReleaseAsset(body)
	if err == nil {
		t.Fatal("expected duplicate signed installer asset error")
	}
	if !strings.Contains(err.Error(), "multiple signed installer packages") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSignedPkgAssetPriority(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{name: "legacy exact", input: ContainerSignedPkgName, want: 3},
		{name: "versioned signed", input: "container-0.10.0-installer-signed.pkg", want: 2},
		{name: "generic signed", input: "container-latest-installer-signed.pkg", want: 2},
		{name: "unsigned", input: "container-installer-unsigned.pkg", want: 0},
		{name: "archive", input: "container-dSYM.zip", want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := signedPkgAssetPriority(tc.input); got != tc.want {
				t.Fatalf("unexpected priority: got %d want %d", got, tc.want)
			}
		})
	}
}

func TestInstallContainerRuntimeUsesBrew(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	var commands []string
	containerInstalled := false

	installerLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			if containerInstalled {
				return "/opt/homebrew/bin/container", nil
			}
			return "", exec.ErrNotFound
		case ContainerBrewBinary:
			return "/opt/homebrew/bin/brew", nil
		case "/opt/homebrew/opt/container/bin/container":
			if containerInstalled {
				return name, nil
			}
			return "", exec.ErrNotFound
		default:
			return "", exec.ErrNotFound
		}
	}
	installerRun = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		if name == ContainerBrewBinary && len(args) >= 2 && args[0] == "install" && args[1] == ContainerBrewFormula {
			containerInstalled = true
		}
		return nil
	}
	installerCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/opt/homebrew/bin/brew" && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			if containerInstalled {
				return []byte("/opt/homebrew/opt/container\n"), nil
			}
			return nil, errors.New("exit status 1")
		}
		return nil, errors.New("unexpected command")
	}
	installerFetchLatestReleaseAsset = func() (releaseAssetInfo, error) {
		t.Fatalf("unexpected latest release fetch during brew install")
		return releaseAssetInfo{}, nil
	}
	installerDownloadFile = func(url, dest string) error {
		t.Fatalf("unexpected download during brew install")
		return nil
	}

	var out bytes.Buffer
	if err := InstallContainerRuntime(&out, RuntimeActionOptions{}); err != nil {
		t.Fatalf("InstallContainerRuntime error: %v", err)
	}
	if len(commands) != 2 {
		t.Fatalf("expected 2 commands, got %d: %v", len(commands), commands)
	}
	if commands[0] != "brew install "+ContainerBrewFormula {
		t.Fatalf("unexpected first command: %s", commands[0])
	}
	if commands[1] != "/opt/homebrew/opt/container/bin/container system start --enable-kernel-install" {
		t.Fatalf("unexpected start command: %s", commands[1])
	}
}

func TestInstallContainerRuntimeStopsSignedRuntimeBeforeBrewInstall(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	var commands []string
	homebrewInstalled := false
	installerLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			if homebrewInstalled {
				return "/opt/homebrew/bin/container", nil
			}
			return "/usr/local/bin/container", nil
		case ContainerBrewBinary:
			return "/opt/homebrew/bin/brew", nil
		case "/opt/homebrew/opt/container/bin/container":
			if homebrewInstalled {
				return name, nil
			}
			return "", exec.ErrNotFound
		default:
			return "", exec.ErrNotFound
		}
	}
	installerCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/opt/homebrew/bin/brew" && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			return []byte("/opt/homebrew/opt/container\n"), nil
		}
		return nil, errors.New("unexpected command")
	}
	installerRun = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		if name == ContainerBrewBinary && len(args) == 2 && args[0] == "install" && args[1] == ContainerBrewFormula {
			homebrewInstalled = true
		}
		return nil
	}
	installerFetchLatestReleaseAsset = func() (releaseAssetInfo, error) {
		t.Fatal("unexpected latest release fetch during brew install")
		return releaseAssetInfo{}, nil
	}

	var out bytes.Buffer
	if err := InstallContainerRuntime(&out, RuntimeActionOptions{}); err != nil {
		t.Fatalf("InstallContainerRuntime error: %v", err)
	}
	want := []string{
		"/usr/local/bin/container system stop",
		"brew install " + ContainerBrewFormula,
		"/opt/homebrew/opt/container/bin/container system start --enable-kernel-install",
	}
	if len(commands) != len(want) {
		t.Fatalf("expected %d commands, got %d: %v", len(want), len(commands), commands)
	}
	for i := range want {
		if commands[i] != want[i] {
			t.Fatalf("unexpected command %d: got %q want %q", i, commands[i], want[i])
		}
	}
}

func TestInstallContainerRuntimeUpgradesOutdatedRuntimeWithBrew(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	var commands []string
	upgraded := false

	installerLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			return "/opt/homebrew/bin/container", nil
		case ContainerBrewBinary:
			return "/opt/homebrew/bin/brew", nil
		case "/opt/homebrew/opt/container/bin/container":
			return name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	installerCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/opt/homebrew/bin/brew" && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			return []byte("/opt/homebrew/opt/container\n"), nil
		}
		if name == "/opt/homebrew/opt/container/bin/container" && len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
			if upgraded {
				return []byte("container version 1.6.0\n"), nil
			}
			return []byte("container version 0.8.0\n"), nil
		}
		return nil, errors.New("unexpected command")
	}
	installerRun = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		if name == ContainerBrewBinary && len(args) == 2 && args[0] == "upgrade" && args[1] == ContainerBrewFormula {
			upgraded = true
		}
		return nil
	}

	var out bytes.Buffer
	if err := InstallContainerRuntime(&out, RuntimeActionOptions{}); err != nil {
		t.Fatalf("InstallContainerRuntime error: %v", err)
	}
	if len(commands) != 3 {
		t.Fatalf("expected 3 commands, got %d: %v", len(commands), commands)
	}
	if commands[0] != "/opt/homebrew/opt/container/bin/container system stop" {
		t.Fatalf("unexpected first command: %s", commands[0])
	}
	if commands[1] != "brew upgrade "+ContainerBrewFormula {
		t.Fatalf("unexpected second command: %s", commands[1])
	}
	if commands[2] != "/opt/homebrew/opt/container/bin/container system start --enable-kernel-install" {
		t.Fatalf("unexpected third command: %s", commands[2])
	}
	output := out.String()
	if !strings.Contains(output, "requires container 1.6.0 or newer; detected 0.8.0") {
		t.Fatalf("expected outdated-version guidance, got: %s", output)
	}
	if !strings.Contains(output, "Upgrading "+ContainerBrewFormula+" via Homebrew") {
		t.Fatalf("expected brew upgrade output, got: %s", output)
	}
}

func TestInstallContainerRuntimeAbortsBrewUpgradeWhenStopFails(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	var commands []string
	installerLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			return "/opt/homebrew/bin/container", nil
		case ContainerBrewBinary:
			return "/opt/homebrew/bin/brew", nil
		case "/opt/homebrew/opt/container/bin/container":
			return name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	installerCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/opt/homebrew/bin/brew" && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			return []byte("/opt/homebrew/opt/container\n"), nil
		}
		if name == "/opt/homebrew/opt/container/bin/container" && len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
			return []byte("container version 0.8.0\n"), nil
		}
		return nil, errors.New("unexpected command")
	}
	installerRun = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return errors.New("stop failed")
	}

	var out bytes.Buffer
	err := InstallContainerRuntime(&out, RuntimeActionOptions{})
	if err == nil || !strings.Contains(err.Error(), "before Homebrew upgrade") {
		t.Fatalf("expected stop-before-upgrade error, got %v", err)
	}
	if len(commands) != 1 || commands[0] != "/opt/homebrew/opt/container/bin/container system stop" {
		t.Fatalf("upgrade should abort after failed stop, commands: %v", commands)
	}
}

func TestInstallContainerRuntimePrefersInstalledHomebrewContainerOverSystemPath(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	var commands []string

	installerLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			return "/usr/local/bin/container", nil
		case ContainerBrewBinary:
			return "/opt/homebrew/bin/brew", nil
		case "/opt/homebrew/opt/container/bin/container":
			return name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	installerCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/opt/homebrew/bin/brew" && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			return []byte("/opt/homebrew/opt/container\n"), nil
		}
		if name == "/opt/homebrew/opt/container/bin/container" && len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
			return []byte("container version 1.6.0\n"), nil
		}
		return nil, errors.New("unexpected command")
	}
	installerRun = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil
	}

	var out bytes.Buffer
	if err := InstallContainerRuntime(&out, RuntimeActionOptions{}); err != nil {
		t.Fatalf("InstallContainerRuntime error: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d: %v", len(commands), commands)
	}
	if commands[0] != "/opt/homebrew/opt/container/bin/container system start --enable-kernel-install" {
		t.Fatalf("unexpected command: %s", commands[0])
	}
	output := out.String()
	if !strings.Contains(output, "container runtime already installed via Homebrew at /opt/homebrew/opt/container/bin/container") {
		t.Fatalf("expected Homebrew path output, got: %s", output)
	}
}

func TestInstallContainerRuntimeOutdatedRuntimeSkipBrewUsesPkg(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	var commands []string
	containerInstalled := false

	installerLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			if containerInstalled {
				return "/usr/local/bin/container", nil
			}
			return "/usr/local/bin/container", nil
		case ContainerBrewBinary:
			return "/opt/homebrew/bin/brew", nil
		default:
			return "", exec.ErrNotFound
		}
	}
	installerCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/usr/local/bin/container" && len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
			if containerInstalled {
				return []byte("container version 1.6.0\n"), nil
			}
			return []byte("container version 0.8.0\n"), nil
		}
		return nil, errors.New("unexpected command")
	}
	installerFetchLatestReleaseAsset = func() (releaseAssetInfo, error) {
		return releaseAssetInfo{
			Tag:  "0.10.0",
			Name: ContainerSignedPkgName,
			URL:  "https://example/" + ContainerSignedPkgName,
		}, nil
	}
	installerMkdirTemp = func(dir, pattern string) (string, error) {
		return "/tmp/ned-test", nil
	}
	installerRemoveAll = func(path string) error {
		return nil
	}
	installerDownloadFile = func(url, dest string) error {
		if url != "https://example/"+ContainerSignedPkgName {
			t.Fatalf("unexpected download URL: %s", url)
		}
		return nil
	}
	installerRun = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		if name == "sudo" {
			containerInstalled = true
		}
		if name == ContainerBrewBinary {
			t.Fatalf("unexpected brew usage during skip-brew")
		}
		return nil
	}

	var out bytes.Buffer
	if err := InstallContainerRuntime(&out, RuntimeActionOptions{SkipBrew: true}); err != nil {
		t.Fatalf("InstallContainerRuntime error: %v", err)
	}
	if len(commands) != 3 {
		t.Fatalf("expected 3 commands, got %d: %v", len(commands), commands)
	}
	if commands[0] != "/usr/local/bin/container system stop" {
		t.Fatalf("unexpected stop command: %s", commands[0])
	}
	if commands[1] != "sudo installer -pkg /tmp/ned-test/"+ContainerSignedPkgName+" -target /" {
		t.Fatalf("unexpected installer command: %s", commands[1])
	}
	if commands[2] != "/usr/local/bin/container system start --enable-kernel-install" {
		t.Fatalf("unexpected final command: %s", commands[2])
	}
}

func TestInstallContainerRuntimeAbortsPkgUpgradeWhenStopFails(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	var commands []string
	installerLookPath = func(name string) (string, error) {
		if name == ContainerRuntimeBinary {
			return "/usr/local/bin/container", nil
		}
		return "", exec.ErrNotFound
	}
	installerCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/usr/local/bin/container" && len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
			return []byte("container version 0.8.0\n"), nil
		}
		return nil, errors.New("unexpected command")
	}
	installerFetchLatestReleaseAsset = func() (releaseAssetInfo, error) {
		return releaseAssetInfo{Name: ContainerSignedPkgName, URL: "https://example/" + ContainerSignedPkgName}, nil
	}
	installerRun = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return errors.New("stop failed")
	}
	installerMkdirTemp = func(dir, pattern string) (string, error) {
		return "/tmp/ned-test", nil
	}
	installerRemoveAll = func(path string) error {
		return nil
	}
	installerDownloadFile = func(url, dest string) error {
		return nil
	}

	var out bytes.Buffer
	err := InstallContainerRuntime(&out, RuntimeActionOptions{SkipBrew: true})
	if err == nil || !strings.Contains(err.Error(), "before pkg upgrade") {
		t.Fatalf("expected stop-before-upgrade error, got %v", err)
	}
	if len(commands) != 1 || commands[0] != "/usr/local/bin/container system stop" {
		t.Fatalf("pkg upgrade should abort after failed stop, commands: %v", commands)
	}
}

func TestInstallContainerRuntimeDoesNotStopPkgRuntimeWhenDownloadFails(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	var commands []string
	installerLookPath = func(name string) (string, error) {
		if name == ContainerRuntimeBinary {
			return "/usr/local/bin/container", nil
		}
		return "", exec.ErrNotFound
	}
	installerCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/usr/local/bin/container" && len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
			return []byte("container version 0.8.0\n"), nil
		}
		return nil, errors.New("unexpected command")
	}
	installerFetchLatestReleaseAsset = func() (releaseAssetInfo, error) {
		return releaseAssetInfo{Name: ContainerSignedPkgName, URL: "https://example/" + ContainerSignedPkgName}, nil
	}
	installerMkdirTemp = func(dir, pattern string) (string, error) {
		return "/tmp/ned-test", nil
	}
	installerRemoveAll = func(path string) error {
		return nil
	}
	installerDownloadFile = func(url, dest string) error {
		return errors.New("download failed")
	}
	installerRun = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil
	}

	var out bytes.Buffer
	err := InstallContainerRuntime(&out, RuntimeActionOptions{SkipBrew: true})
	if err == nil || !strings.Contains(err.Error(), "download failed") {
		t.Fatalf("expected download failure, got %v", err)
	}
	if len(commands) != 0 {
		t.Fatalf("runtime must remain running when the pkg download fails, commands: %v", commands)
	}
}

func TestInstallContainerRuntimeFallsBackToPkg(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	var commands []string
	containerInstalled := false
	installerLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			if containerInstalled {
				return "/usr/local/bin/container", nil
			}
			return "", exec.ErrNotFound
		case ContainerBrewBinary:
			return "", exec.ErrNotFound
		default:
			return "", exec.ErrNotFound
		}
	}
	installerFetchLatestReleaseAsset = func() (releaseAssetInfo, error) {
		return releaseAssetInfo{
			Tag:  "0.8.0",
			Name: ContainerSignedPkgName,
			URL:  "https://example/" + ContainerSignedPkgName,
		}, nil
	}
	installerMkdirTemp = func(dir, pattern string) (string, error) {
		return "/tmp/ned-test", nil
	}
	installerRemoveAll = func(path string) error {
		return nil
	}
	var downloaded string
	installerDownloadFile = func(url, dest string) error {
		downloaded = dest
		if url != "https://example/"+ContainerSignedPkgName {
			t.Fatalf("unexpected download URL: %s", url)
		}
		return nil
	}
	installerRun = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		if name == "sudo" {
			containerInstalled = true
		}
		return nil
	}

	var out bytes.Buffer
	if err := InstallContainerRuntime(&out, RuntimeActionOptions{}); err != nil {
		t.Fatalf("InstallContainerRuntime error: %v", err)
	}
	if downloaded != "/tmp/ned-test/"+ContainerSignedPkgName {
		t.Fatalf("unexpected download destination: %s", downloaded)
	}
	if len(commands) != 2 {
		t.Fatalf("expected 2 commands, got %d: %v", len(commands), commands)
	}
	if commands[0] != "sudo installer -pkg /tmp/ned-test/"+ContainerSignedPkgName+" -target /" {
		t.Fatalf("unexpected installer command: %s", commands[0])
	}
	if commands[1] != "/usr/local/bin/container system start --enable-kernel-install" {
		t.Fatalf("unexpected start command: %s", commands[1])
	}
	output := out.String()
	if !strings.Contains(output, "Homebrew not found") {
		t.Fatalf("expected Homebrew missing message, got: %s", output)
	}
	if !strings.Contains(output, ContainerReleasePageURL) {
		t.Fatalf("expected release page in output, got: %s", output)
	}
	if !strings.Contains(output, ContainerSignedPkgHint) {
		t.Fatalf("expected signed pkg placeholder in output, got: %s", output)
	}
}

func TestInstallContainerRuntimeDryRunUsesBrew(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	installerLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			return "", exec.ErrNotFound
		case ContainerBrewBinary:
			return "/opt/homebrew/bin/brew", nil
		default:
			return "", exec.ErrNotFound
		}
	}
	installerRun = func(name string, args ...string) error {
		t.Fatalf("unexpected command execution during dry-run: %s %v", name, args)
		return nil
	}
	installerCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/opt/homebrew/bin/brew" && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			return nil, errors.New("exit status 1")
		}
		return nil, errors.New("unexpected command")
	}

	var out bytes.Buffer
	if err := InstallContainerRuntime(&out, RuntimeActionOptions{DryRun: true}); err != nil {
		t.Fatalf("InstallContainerRuntime error: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "DRY RUN: brew install "+ContainerBrewFormula) {
		t.Fatalf("expected dry-run brew install output, got: %s", output)
	}
	if !strings.Contains(output, "DRY RUN: "+ContainerRuntimeBinary+" system start --enable-kernel-install") {
		t.Fatalf("expected dry-run system start output, got: %s", output)
	}
}

func TestInstallContainerRuntimeSkipBrew(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	var commands []string
	containerInstalled := false

	installerLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			if containerInstalled {
				return "/usr/local/bin/container", nil
			}
			return "", exec.ErrNotFound
		case ContainerBrewBinary:
			return "/opt/homebrew/bin/brew", nil
		default:
			return "", exec.ErrNotFound
		}
	}
	installerFetchLatestReleaseAsset = func() (releaseAssetInfo, error) {
		return releaseAssetInfo{
			Tag:  "0.8.0",
			Name: ContainerSignedPkgName,
			URL:  "https://example/" + ContainerSignedPkgName,
		}, nil
	}
	installerMkdirTemp = func(dir, pattern string) (string, error) {
		return "/tmp/ned-test", nil
	}
	installerRemoveAll = func(path string) error {
		return nil
	}
	installerDownloadFile = func(url, dest string) error {
		if url != "https://example/"+ContainerSignedPkgName {
			t.Fatalf("unexpected download URL: %s", url)
		}
		return nil
	}
	installerRun = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		if name == "sudo" {
			containerInstalled = true
		}
		if name == ContainerBrewBinary {
			t.Fatalf("unexpected brew usage during skip-brew")
		}
		return nil
	}

	var out bytes.Buffer
	if err := InstallContainerRuntime(&out, RuntimeActionOptions{SkipBrew: true}); err != nil {
		t.Fatalf("InstallContainerRuntime error: %v", err)
	}
	if len(commands) != 2 {
		t.Fatalf("expected 2 commands, got %d: %v", len(commands), commands)
	}
	if commands[0] != "sudo installer -pkg /tmp/ned-test/"+ContainerSignedPkgName+" -target /" {
		t.Fatalf("unexpected installer command: %s", commands[0])
	}
	if commands[1] != "/usr/local/bin/container system start --enable-kernel-install" {
		t.Fatalf("unexpected start command: %s", commands[1])
	}
	output := out.String()
	if !strings.Contains(output, "Skipping Homebrew") {
		t.Fatalf("expected skip-brew output, got: %s", output)
	}
}

func TestInstallContainerRuntimeUsesSudoOverride(t *testing.T) {
	restore := saveInstallerDeps()
	defer restore()

	t.Setenv(installerTestSudoEnv, "doas")

	var commands []string
	containerInstalled := false
	installerLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			if containerInstalled {
				return "/usr/local/bin/container", nil
			}
			return "", exec.ErrNotFound
		case ContainerBrewBinary:
			return "", exec.ErrNotFound
		default:
			return "", exec.ErrNotFound
		}
	}
	installerFetchLatestReleaseAsset = func() (releaseAssetInfo, error) {
		return releaseAssetInfo{
			Tag:  "0.8.0",
			Name: ContainerSignedPkgName,
			URL:  "https://example/" + ContainerSignedPkgName,
		}, nil
	}
	installerMkdirTemp = func(dir, pattern string) (string, error) {
		return "/tmp/ned-test", nil
	}
	installerRemoveAll = func(path string) error {
		return nil
	}
	installerDownloadFile = func(url, dest string) error {
		return nil
	}
	installerRun = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		if name == "doas" {
			containerInstalled = true
		}
		return nil
	}

	var out bytes.Buffer
	if err := InstallContainerRuntime(&out, RuntimeActionOptions{SkipBrew: true}); err != nil {
		t.Fatalf("InstallContainerRuntime error: %v", err)
	}
	if len(commands) != 2 {
		t.Fatalf("expected 2 commands, got %d: %v", len(commands), commands)
	}
	if commands[0] != "doas installer -pkg /tmp/ned-test/"+ContainerSignedPkgName+" -target /" {
		t.Fatalf("unexpected installer command: %s", commands[0])
	}
}

func saveInstallerDeps() func() {
	origLookPath := installerLookPath
	origRun := installerRun
	origCommandOutput := installerCommandOutput
	origHTTPGet := installerHTTPGet
	origMkdirTemp := installerMkdirTemp
	origRemoveAll := installerRemoveAll
	origDownloadFile := installerDownloadFile
	origFetchLatest := installerFetchLatestReleaseAsset
	return func() {
		installerLookPath = origLookPath
		installerRun = origRun
		installerCommandOutput = origCommandOutput
		installerHTTPGet = origHTTPGet
		installerMkdirTemp = origMkdirTemp
		installerRemoveAll = origRemoveAll
		installerDownloadFile = origDownloadFile
		installerFetchLatestReleaseAsset = origFetchLatest
	}
}
