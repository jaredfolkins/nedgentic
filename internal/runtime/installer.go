package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	ContainerRuntimeBinary  = "container"
	ContainerBrewBinary     = "brew"
	ContainerBrewFormula    = "container"
	ContainerReleaseAPIURL  = "https://api.github.com/repos/apple/container/releases/latest"
	ContainerReleasePageURL = "https://github.com/apple/container/releases"
	ContainerSignedPkgName  = "container-installer-signed.pkg"
	ContainerSignedPkgHint  = "<downloaded-signed-installer.pkg>"
	ContainerUninstallPath  = "/usr/local/bin/uninstall-container.sh"

	installerTestSudoEnv          = "NED_TEST_INSTALLER_SUDO"
	installerTestUninstallPathEnv = "NED_TEST_INSTALLER_UNINSTALL_PATH"
)

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type releaseResponse struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAssetInfo struct {
	Tag  string
	Name string
	URL  string
}

type RuntimeActionOptions struct {
	DryRun   bool
	SkipBrew bool
}

var (
	installerLookPath                = exec.LookPath
	installerRun                     = runCommandStreaming
	installerCommandOutput           = runtimeCommandOutput
	installerHTTPGet                 = defaultHTTPGet
	installerMkdirTemp               = os.MkdirTemp
	installerRemoveAll               = os.RemoveAll
	installerDownloadFile            = downloadFile
	installerFetchLatestReleaseAsset = fetchLatestReleaseAsset
	defaultInstallerHTTPClient       = &http.Client{Timeout: 30 * time.Second}
)

func InstallContainerRuntime(out io.Writer, opts RuntimeActionOptions) error {
	if out == nil {
		out = io.Discard
	}
	if !opts.SkipBrew {
		if brewPath, err := installerLookPath(ContainerBrewBinary); err == nil {
			fmt.Fprintf(out, "Homebrew detected at %s\n", brewPath)
			if path, err := resolveHomebrewContainerRuntimePath(installerLookPath, installerCommandOutput); err == nil {
				needsUpgrade, reason := installerContainerNeedsUpgrade(path)
				if !needsUpgrade {
					fmt.Fprintf(out, "container runtime already installed via Homebrew at %s\n", path)
					return StartContainerRuntime(out, path, opts)
				}
				fmt.Fprintf(out, "container runtime found via Homebrew at %s but %s\n", path, reason)
				if err := StopContainerRuntime(out, path, opts); err != nil {
					return fmt.Errorf("failed to stop existing container runtime before Homebrew upgrade: %w", err)
				}
				fmt.Fprintf(out, "Upgrading %s via Homebrew...\n", ContainerBrewFormula)
				if err := runOrPrint(out, opts, ContainerBrewBinary, "upgrade", ContainerBrewFormula); err != nil {
					return fmt.Errorf("brew upgrade failed: %v", err)
				}
				return StartContainerRuntime(out, "", opts)
			}
			if path, err := installerLookPath(ContainerRuntimeBinary); err == nil {
				fmt.Fprintf(out, "container runtime found outside Homebrew at %s; stopping it before Homebrew install.\n", path)
				if err := StopContainerRuntime(out, path, opts); err != nil {
					return fmt.Errorf("failed to stop existing container runtime before Homebrew install: %w", err)
				}
			}
			fmt.Fprintf(out, "Installing %s via Homebrew...\n", ContainerBrewFormula)
			if err := runOrPrint(out, opts, ContainerBrewBinary, "install", ContainerBrewFormula); err != nil {
				return fmt.Errorf("brew install failed: %v", err)
			}
			return StartContainerRuntime(out, "", opts)
		}
	} else {
		fmt.Fprintln(out, "Skipping Homebrew as requested; installing from the signed pkg.")
	}

	containerNeedsUpgrade := false
	localContainerPath := ""
	if path, err := installerLookPath(ContainerRuntimeBinary); err == nil {
		localContainerPath = path
		needsUpgrade, reason := installerContainerNeedsUpgrade(path)
		if !needsUpgrade {
			fmt.Fprintf(out, "container runtime already installed at %s\n", path)
			return StartContainerRuntime(out, path, opts)
		}
		containerNeedsUpgrade = true
		fmt.Fprintf(out, "container runtime found at %s but %s\n", path, reason)
	}

	fmt.Fprintln(out, "Homebrew not found. Installing from the latest signed Apple release.")
	fmt.Fprintf(out, "Manual install steps:\n")
	fmt.Fprintf(out, "  1) Download the latest signed installer from %s\n", ContainerReleasePageURL)
	fmt.Fprintf(out, "  2) sudo installer -pkg %s -target /\n", ContainerSignedPkgHint)
	fmt.Fprintf(out, "  3) container system start --enable-kernel-install\n")

	asset, err := installerFetchLatestReleaseAsset()
	if err != nil {
		return err
	}
	if strings.TrimSpace(asset.URL) == "" {
		return fmt.Errorf("latest release asset URL is empty")
	}

	safeName := filepath.Base(asset.Name)
	if safeName != asset.Name {
		return fmt.Errorf("unexpected asset name %q", asset.Name)
	}
	pkgPath := filepath.Join(os.TempDir(), safeName)
	if opts.DryRun {
		fmt.Fprintf(out, "DRY RUN: download %s -> %s\n", asset.URL, pkgPath)
		if containerNeedsUpgrade {
			if err := StopContainerRuntime(out, localContainerPath, opts); err != nil {
				return fmt.Errorf("failed to stop existing container runtime before pkg upgrade: %w", err)
			}
		}
		fmt.Fprintf(out, "Installing %s (sudo required)...\n", pkgPath)
		if err := runOrPrint(out, opts, installerSudoCommand(), "installer", "-pkg", pkgPath, "-target", "/"); err != nil {
			return fmt.Errorf("installer failed: %v", err)
		}
		return startInstalledLocalContainerRuntime(out, opts)
	}

	tmpDir, err := installerMkdirTemp("", "ned-container-install-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = installerRemoveAll(tmpDir)
	}()

	pkgPath = filepath.Join(tmpDir, safeName)
	fmt.Fprintf(out, "Downloading %s...\n", asset.URL)
	if err := installerDownloadFile(asset.URL, pkgPath); err != nil {
		return err
	}
	if containerNeedsUpgrade {
		if err := StopContainerRuntime(out, localContainerPath, opts); err != nil {
			return fmt.Errorf("failed to stop existing container runtime before pkg upgrade: %w", err)
		}
	}

	fmt.Fprintf(out, "Installing %s (sudo required)...\n", pkgPath)
	if err := runOrPrint(out, opts, installerSudoCommand(), "installer", "-pkg", pkgPath, "-target", "/"); err != nil {
		return fmt.Errorf("installer failed: %v", err)
	}
	return startInstalledLocalContainerRuntime(out, opts)
}

func startInstalledLocalContainerRuntime(out io.Writer, opts RuntimeActionOptions) error {
	if strings.TrimSpace(ContainerRuntimeBinary) == "" {
		return fmt.Errorf("container runtime binary is not configured")
	}
	if opts.DryRun {
		return StartContainerRuntime(out, ContainerRuntimeBinary, opts)
	}
	path, err := installerLookPath(ContainerRuntimeBinary)
	if err != nil {
		return fmt.Errorf("container runtime not found in PATH after local install: %v", err)
	}
	return StartContainerRuntime(out, path, opts)
}

func installerContainerNeedsUpgrade(path string) (bool, string) {
	versionOutput, err := installerCommandOutput(path, "--version")
	if err != nil {
		return true, fmt.Sprintf("version check failed: %v; upgrading to ensure compatibility", err)
	}
	if err := checkContainerVersion(string(versionOutput)); err != nil {
		return true, err.Error()
	}
	return false, ""
}

func StopContainerRuntime(out io.Writer, bin string, opts RuntimeActionOptions) error {
	if out == nil {
		out = io.Discard
	}
	if strings.TrimSpace(bin) == "" {
		if opts.DryRun {
			bin = ContainerRuntimeBinary
		} else {
			resolved, err := resolveContainerRuntimePath("", installerLookPath, installerCommandOutput)
			if err != nil {
				// If not found, we can't stop it, but maybe it's already gone?
				// Treat as success or error? Treating as error to be safe,
				// but caller might handle "not found".
				return fmt.Errorf("container runtime not found in PATH: %v", err)
			}
			bin = resolved
		}
	}
	fmt.Fprintf(out, "Stopping container runtime with %s...\n", bin)
	if err := runOrPrint(out, opts, bin, "system", "stop"); err != nil {
		return fmt.Errorf("failed to stop container runtime: %v", err)
	}
	return nil
}

func StartContainerRuntime(out io.Writer, bin string, opts RuntimeActionOptions) error {
	if out == nil {
		out = io.Discard
	}
	if strings.TrimSpace(bin) == "" {
		if opts.DryRun {
			bin = ContainerRuntimeBinary
		} else {
			resolved, err := resolveContainerRuntimePath("", installerLookPath, installerCommandOutput)
			if err != nil {
				return fmt.Errorf("container runtime not found in PATH after install: %v", err)
			}
			bin = resolved
		}
	}
	fmt.Fprintf(out, "Starting container runtime with %s...\n", bin)
	if err := runOrPrint(out, opts, bin, "system", "start", "--enable-kernel-install"); err != nil {
		return fmt.Errorf("failed to start container runtime: %v", err)
	}
	return nil
}

func defaultHTTPGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "nedgentic")
	req.Header.Set("Accept", "application/vnd.github+json")
	return defaultInstallerHTTPClient.Do(req)
}

func fetchLatestReleaseAsset() (releaseAssetInfo, error) {
	resp, err := installerHTTPGet(ContainerReleaseAPIURL)
	if err != nil {
		return releaseAssetInfo{}, fmt.Errorf("failed to fetch latest release: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return releaseAssetInfo{}, fmt.Errorf("failed to fetch latest release: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return releaseAssetInfo{}, fmt.Errorf("failed to read latest release response: %v", err)
	}
	return parseLatestReleaseAsset(body)
}

func parseLatestReleaseAsset(body []byte) (releaseAssetInfo, error) {
	var release releaseResponse
	if err := json.Unmarshal(body, &release); err != nil {
		return releaseAssetInfo{}, fmt.Errorf("failed to parse latest release: %v", err)
	}
	bestPriority := 0
	var best releaseAssetInfo
	var duplicate string
	for _, asset := range release.Assets {
		priority := signedPkgAssetPriority(asset.Name)
		if priority == 0 {
			continue
		}
		if strings.TrimSpace(asset.BrowserDownloadURL) == "" {
			return releaseAssetInfo{}, fmt.Errorf("latest release asset %q has empty download URL", asset.Name)
		}
		candidate := releaseAssetInfo{
			Tag:  release.TagName,
			Name: asset.Name,
			URL:  asset.BrowserDownloadURL,
		}
		if priority > bestPriority {
			bestPriority = priority
			best = candidate
			duplicate = ""
			continue
		}
		if priority == bestPriority {
			duplicate = asset.Name
		}
	}
	if duplicate != "" {
		return releaseAssetInfo{}, fmt.Errorf("latest release includes multiple signed installer packages (%q, %q); see %s", best.Name, duplicate, ContainerReleasePageURL)
	}
	if bestPriority == 0 {
		return releaseAssetInfo{}, fmt.Errorf("latest release does not include a signed installer package matching %q (see %s)", ContainerSignedPkgName, ContainerReleasePageURL)
	}
	return best, nil
}

func signedPkgAssetPriority(name string) int {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return 0
	}
	if !strings.HasPrefix(lower, "container") || !strings.HasSuffix(lower, ".pkg") {
		return 0
	}
	if strings.Contains(lower, "unsigned") || strings.Contains(lower, "dsym") {
		return 0
	}
	if lower == ContainerSignedPkgName {
		return 3
	}
	if strings.HasPrefix(lower, "container-") && strings.HasSuffix(lower, "-installer-signed.pkg") {
		return 2
	}
	if strings.Contains(lower, "installer-signed.pkg") {
		return 1
	}
	return 0
}

func downloadFile(url, dest string) error {
	resp, err := installerHTTPGet(url)
	if err != nil {
		return fmt.Errorf("download failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: status %d", resp.StatusCode)
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create download file: %v", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write download file: %v", err)
	}
	return nil
}

func missingContainerRuntimeError(bin string, cause error) error {
	trimmed := strings.TrimSpace(bin)
	if trimmed == "" {
		trimmed = ContainerRuntimeBinary
	}
	base := fmt.Sprintf("container runtime %q not found", trimmed)
	if cause != nil {
		base = fmt.Sprintf("%s: %v", base, cause)
	}
	return fmt.Errorf("%s. Rerun ned to install it, or install it manually: `brew install %s` or download the signed installer from %s and run `sudo installer -pkg %s -target /`, then `container system start --enable-kernel-install`.", base, ContainerBrewFormula, ContainerReleasePageURL, ContainerSignedPkgHint)
}

func runOrPrint(out io.Writer, opts RuntimeActionOptions, name string, args ...string) error {
	if out == nil {
		out = io.Discard
	}
	if opts.DryRun {
		fmt.Fprintf(out, "DRY RUN: %s\n", commandString(name, args...))
		return nil
	}
	return installerRun(name, args...)
}

func installerSudoCommand() string {
	if override := strings.TrimSpace(os.Getenv(installerTestSudoEnv)); override != "" {
		return override
	}
	return "sudo"
}

func commandString(name string, args ...string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

func runCommandStreaming(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
