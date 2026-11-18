package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
	"nedgentic/internal/scripts"
)

func ResolveContainerBin(flags profile.Flags) (string, error) {
	candidate := strings.TrimSpace(flags.ContainerBin)
	path, err := resolveContainerRuntimePath(candidate, exec.LookPath, statusCommandOutput)
	if err != nil {
		return "", err
	}
	if err := RequireContainerRuntimeVersion(path); err != nil {
		return "", err
	}
	if _, err := EnsureContainerRuntimeStarted(path); err != nil {
		return "", err
	}
	return path, nil
}

func CheckContainerRuntime(bin string) error {
	path, err := exec.LookPath(bin)
	if err != nil {
		return missingContainerRuntimeError(bin, err)
	}
	return RequireContainerRuntimeVersion(path)
}

func containerCommandOutput(bin string, args ...string) ([]byte, error) {
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}

func containerImageExists(bin, image string) (bool, error) {
	if strings.TrimSpace(image) == "" {
		return false, fmt.Errorf("image name is required")
	}
	_, err := containerCommandOutput(bin, "image", "inspect", image)
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, err
}

func ContainerImageExists(bin, image string) (bool, error) {
	return containerImageExists(bin, image)
}

func ImagePrune(bin string, all bool) error {
	args := []string{"image", "prune"}
	if all {
		args = append(args, "--all")
	}
	out, err := containerCommandOutput(bin, args...)
	if err != nil {
		return fmt.Errorf("image prune failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ContainerPrune(bin string) error {
	out, err := containerCommandOutput(bin, "prune")
	if err != nil {
		return fmt.Errorf("container prune failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func BuilderDelete(bin string) error {
	out, err := containerCommandOutput(bin, "builder", "delete", "--force")
	if err != nil {
		return fmt.Errorf("builder delete failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

type ContainerListEntry struct {
	Status        string                     `json:"status"`
	Networks      []ContainerListNetwork     `json:"networks"`
	Configuration ContainerListConfiguration `json:"configuration"`
}

type ContainerListNetwork struct {
	IPv4Address string `json:"ipv4Address"`
}

type ContainerListConfiguration struct {
	ID     string            `json:"id"`
	Labels map[string]string `json:"labels"`
	Image  struct {
		Reference string `json:"reference"`
	} `json:"image"`
}

func (entry *ContainerListEntry) UnmarshalJSON(data []byte) error {
	var raw struct {
		Status        json.RawMessage            `json:"status"`
		Networks      []ContainerListNetwork     `json:"networks"`
		Configuration ContainerListConfiguration `json:"configuration"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	status, statusNetworks, err := parseContainerStatusField(raw.Status)
	if err != nil {
		return err
	}
	entry.Status = status
	entry.Networks = raw.Networks
	if len(entry.Networks) == 0 {
		entry.Networks = statusNetworks
	}
	entry.Configuration = raw.Configuration
	return nil
}

func parseContainerStatusField(data []byte) (string, []ContainerListNetwork, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return "", nil, nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		return text, nil, nil
	}
	var object struct {
		State    string                 `json:"state"`
		Status   string                 `json:"status"`
		Networks []ContainerListNetwork `json:"networks"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return "", nil, err
	}
	value := strings.TrimSpace(object.State)
	if value == "" {
		value = strings.TrimSpace(object.Status)
	}
	return value, object.Networks, nil
}

type containerConfigInspectEntry struct {
	Configuration json.RawMessage `json:"configuration"`
}

func sanitizeJSONOutput(output []byte) []byte {
	s := strings.TrimSpace(string(SanitizeStructuredCommandOutput(output)))
	if idx := strings.IndexAny(s, "[{"); idx != -1 {
		return []byte(s[idx:])
	}
	return []byte(s)
}

func extractContainerConfiguration(output []byte) (json.RawMessage, error) {
	output = sanitizeJSONOutput(output)
	var entries []containerConfigInspectEntry
	if err := json.Unmarshal(output, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse container inspect: %v", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("container inspect returned no entries")
	}
	if len(entries[0].Configuration) == 0 {
		return nil, fmt.Errorf("container inspect returned empty configuration")
	}
	return entries[0].Configuration, nil
}

func ListContainers(bin string) ([]ContainerListEntry, error) {
	out, err := containerCommandOutput(bin, "list", "--all", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("container list failed: %w", FormatCommandOutputError(err, out))
	}
	return parseContainerListOutput(out)
}

func parseContainerListOutput(out []byte) ([]ContainerListEntry, error) {
	out = sanitizeJSONOutput(out)
	var entries []ContainerListEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse container list: %v", err)
	}
	return entries, nil
}

func ContainerStatusByName(bin, name string) (bool, string, error) {
	entries, err := ListContainers(bin)
	if err != nil {
		return false, "", err
	}
	for _, entry := range entries {
		if entry.Configuration.ID == name {
			return true, entry.Status, nil
		}
	}
	return false, "", nil
}

const (
	appleContainerDirName        = "com.apple.container"
	appleContainerContainersDir  = "containers"
	appleContainerLibraryDir     = "Library"
	appleContainerSupportDir     = "Application Support"
	containerConfigFilename      = "config.json"
	deleteMissingCocoaMarker     = "NSCocoaErrorDomain Code=260"
	deleteMissingFileMarker      = "No such file or directory"
	deleteMissingNSFilePathToken = "NSFilePath="
)

func ContainerDelete(bin, name string) error {
	out, err := containerCommandOutput(bin, "delete", name)
	if err == nil {
		return nil
	}
	output := strings.TrimSpace(string(out))
	if isMissingContainerConfigDeleteError(output) {
		exists, _, statusErr := ContainerStatusByName(bin, name)
		if statusErr != nil {
			return fmt.Errorf("container delete failed: %v: %s", err, output)
		}
		if !exists {
			if cleanupErr := cleanupMissingContainerConfig(name, output); cleanupErr != nil {
				logging.Warnf("Failed to clean container %s after missing config: %v", name, cleanupErr)
			}
			return nil
		}
		if recoverErr := recoverMissingContainerConfig(bin, name); recoverErr != nil {
			return fmt.Errorf("container delete failed after config recovery attempt: %v", recoverErr)
		}
		out, retryErr := containerCommandOutput(bin, "delete", name)
		if retryErr == nil {
			return nil
		}
		retryOutput := strings.TrimSpace(string(out))
		return fmt.Errorf("container delete failed after config recovery: %v: %s", retryErr, retryOutput)
	}
	return fmt.Errorf("container delete failed: %v: %s", err, output)
}

func ContainerStop(bin, name string) error {
	out, err := containerCommandOutput(bin, "stop", name)
	if err != nil {
		return fmt.Errorf("container stop failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ContainerStart(bin, name string) error {
	out, err := containerCommandOutput(bin, "start", name)
	if err != nil {
		return fmt.Errorf("container start failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isMissingContainerConfigDeleteError(output string) bool {
	if output == "" {
		return false
	}
	if !strings.Contains(output, containerConfigFilename) {
		return false
	}
	return strings.Contains(output, deleteMissingFileMarker) || strings.Contains(output, deleteMissingCocoaMarker)
}

func inspectContainerConfiguration(bin, name string) (json.RawMessage, error) {
	out, err := containerCommandOutput(bin, "inspect", name)
	if err != nil {
		return nil, fmt.Errorf("container inspect failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return extractContainerConfiguration(out)
}

func recoverMissingContainerConfig(bin, name string) error {
	config, err := inspectContainerConfiguration(bin, name)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir, ok := containerStateDirFromName(name, home)
	if !ok {
		return fmt.Errorf("invalid container name %q", name)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, containerConfigFilename)
	if err := os.WriteFile(path, config, 0644); err != nil {
		return err
	}
	return nil
}

func cleanupMissingContainerConfig(name, output string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir, ok := containerStateDirFromOutput(output, name, home)
	if !ok {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func containerStateDirFromOutput(output, name, home string) (string, bool) {
	if strings.TrimSpace(home) == "" {
		return "", false
	}
	if configPath, ok := parseContainerConfigPath(output); ok {
		cleaned := filepath.Clean(configPath)
		if filepath.Base(cleaned) != containerConfigFilename {
			return "", false
		}
		dir := filepath.Dir(cleaned)
		if strings.TrimSpace(name) != "" && filepath.Base(dir) != name {
			return "", false
		}
		base := filepath.Clean(containerStateRoot(home))
		if !strings.HasPrefix(dir+string(os.PathSeparator), base+string(os.PathSeparator)) {
			return "", false
		}
		return dir, true
	}
	return containerStateDirFromName(name, home)
}

func parseContainerConfigPath(output string) (string, bool) {
	return extractPathAfter(output, deleteMissingNSFilePathToken)
}

func extractPathAfter(output, marker string) (string, bool) {
	idx := strings.Index(output, marker)
	if idx == -1 {
		return "", false
	}
	start := idx + len(marker)
	end := len(output)
	if cut := strings.IndexAny(output[start:], ",}"); cut != -1 {
		end = start + cut
	}
	path := strings.TrimSpace(output[start:end])
	path = strings.Trim(path, "\"")
	if path == "" {
		return "", false
	}
	return path, true
}

func containerStateDirFromName(name, home string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", false
	}
	if strings.Contains(trimmed, "..") {
		return "", false
	}
	if strings.ContainsAny(trimmed, `/\`) {
		return "", false
	}
	return filepath.Join(containerStateRoot(home), trimmed), true
}

func containerStateRoot(home string) string {
	return filepath.Join(home, appleContainerLibraryDir, appleContainerSupportDir, appleContainerDirName, appleContainerContainersDir)
}

func ExecContainerShell(opts ExecShellOptions) error {
	shellScript, err := shellScriptWithFallback(opts.ShellPath, opts.Workdir)
	if err != nil {
		return err
	}
	if opts.HostUser == nil {
		return fmt.Errorf("host user is required for container exec")
	}
	env := GuestUserEnv(opts.HostUser, opts.ShellPath, opts.Flags)
	uid := opts.HostUser.UID
	gid := opts.HostUser.GID
	args, err := BuildExecArgs(ExecOptions{
		TTY:           WantsTTY(),
		UID:           &uid,
		GID:           &gid,
		Env:           env,
		Ulimits:       opts.Ulimits,
		Workdir:       opts.Workdir,
		ContainerName: opts.ContainerName,
		Command:       []string{"/bin/sh", "-c", shellScript},
	})
	if err != nil {
		return err
	}

	return RunContainerExec(opts.Bin, args)
}

// GuestUserEnv is the environment for commands run in the VM as the host
// user: the same identity, shell, and proxy settings as an attached shell.
func GuestUserEnv(hostUser *HostUser, shellPath string, flags profile.Flags) []string {
	env := []string{
		"HOME=" + GuestHomeForUser(hostUser.Name),
		"USER=" + hostUser.Name,
		"LOGNAME=" + hostUser.Name,
		"SHELL=" + shellPath,
	}
	return append(env, ProxyEnvEntries(flags)...)
}

func GuestHomeForUser(username string) string {
	return filepath.Join("/home", username)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsRune(value, '\'') {
		return "'" + value + "'"
	}
	return "'" + strings.ReplaceAll(value, "'", `''`) + "'"
}

func shellScriptWithFallback(shellPath, workdir string) (string, error) {
	data := scripts.ShellData{
		ShellPath: shellQuote(shellPath),
	}
	if workdir != "" {
		data.Workdir = shellQuote(workdir)
	}
	return scripts.RenderShell(data)
}

func prepareRunnerScript(hostUser *HostUser, shellPath, workdir string, plan *MountPlan, keepAlive bool, systemd bool, runnerDir string) (string, func(), error) {
	if hostUser == nil {
		return "", nil, fmt.Errorf("host user is required")
	}
	dir := strings.TrimSpace(runnerDir)
	cleanup := func() {}
	if dir == "" {
		var err error
		dir, err = os.MkdirTemp("", "ned-runner-")
		if err != nil {
			return "", nil, err
		}
		cleanup = func() {
			_ = os.RemoveAll(dir)
		}
		if err := os.Chmod(dir, 0700); err != nil {
			cleanup()
			return "", nil, err
		}
	} else {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", nil, err
		}
		if err := os.Chmod(dir, 0700); err != nil {
			return "", nil, err
		}
	}
	scriptName := RunnerScriptName
	content, err := genticRunnerScript(hostUser, shellPath, workdir, plan, keepAlive)
	if systemd {
		scriptName = BootstrapScriptName
		content, err = genticBootstrapScript(hostUser, shellPath, workdir, plan)
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}
	path := filepath.Join(dir, scriptName)
	if err := os.WriteFile(path, []byte(content), 0700); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := os.Chmod(path, 0700); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func genticRunnerScript(hostUser *HostUser, shellPath, workdir string, plan *MountPlan, keepAlive bool) (string, error) {
	if hostUser == nil {
		return "", fmt.Errorf("host user is required")
	}
	shellScript, err := shellScriptWithFallback(shellPath, workdir)
	if err != nil {
		return "", err
	}

	copyBlock := ""
	if plan != nil && len(plan.CopySpecs) > 0 {
		var builder strings.Builder
		for _, spec := range plan.CopySpecs {
			srcPath := filepath.ToSlash(filepath.Join(SecretsMountPath, spec.SourceRel))
			mode := fmt.Sprintf("%04o", spec.Mode.Perm())
			dest := spec.DestPath
			if spec.IsDir {
				builder.WriteString(fmt.Sprintf("if [ -d %s ]; then\n", shellQuote(srcPath)))
				builder.WriteString(fmt.Sprintf("  mkdir -p %s\n", shellQuote(dest)))
				builder.WriteString(fmt.Sprintf("  if cp -a %s/. %s/ 2>/dev/null; then true; else cp -R %s/. %s/; fi\n", shellQuote(srcPath), shellQuote(dest), shellQuote(srcPath), shellQuote(dest)))
				builder.WriteString(fmt.Sprintf("  chown -R \"$USER_UID\":\"$USER_GID\" %s || true\n", shellQuote(dest)))
				builder.WriteString("fi\n")
				continue
			}
			builder.WriteString(fmt.Sprintf("if [ -f %s ]; then\n", shellQuote(srcPath)))
			builder.WriteString(fmt.Sprintf("  mkdir -p \"$(dirname %s)\"\n", shellQuote(dest)))
			builder.WriteString(fmt.Sprintf("  cp %s %s\n", shellQuote(srcPath), shellQuote(dest)))
			builder.WriteString(fmt.Sprintf("  chmod %s %s\n", mode, shellQuote(dest)))
			builder.WriteString(fmt.Sprintf("  chown \"$USER_UID\":\"$USER_GID\" %s || true\n", shellQuote(dest)))
			builder.WriteString("fi\n")
		}
		builder.WriteString("if [ -d \"$HOME_DIR/.ssh\" ]; then chmod 700 \"$HOME_DIR/.ssh\"; chown \"$USER_UID\":\"$USER_GID\" \"$HOME_DIR/.ssh\" || true; fi\n")
		copyBlock = builder.String()
	}

	agentAuthBlock, err := scripts.RenderAgentAuth(shellQuote(AgentAuthMountDir))
	if err != nil {
		return "", err
	}

	shellCommand := shellQuote(shellScript)
	if keepAlive {
		shellCommand = "'tail -f /dev/null'"
	}

	data := scripts.RunnerData{
		UserName:       shellQuote(hostUser.Name),
		UserUID:        hostUser.UID,
		UserGID:        hostUser.GID,
		HomeDir:        shellQuote(GuestHomeForUser(hostUser.Name)),
		ShellPath:      shellQuote(shellPath),
		GenticDir:      shellQuote(GenticMountDir),
		CopyBlock:      copyBlock,
		ShellCommand:   shellCommand,
		AgentAuthBlock: agentAuthBlock,
		WorkspacePath: func() string {
			if plan == nil || strings.TrimSpace(plan.WorkspacePath) == "" {
				return ""
			}
			return shellQuote(plan.WorkspacePath)
		}(),
		WorkspaceMount: func() string {
			if plan == nil || strings.TrimSpace(plan.WorkspaceMount) == "" {
				return ""
			}
			return shellQuote(plan.WorkspaceMount)
		}(),
	}
	return scripts.RenderRunner(data)
}

func genticBootstrapScript(hostUser *HostUser, shellPath, workdir string, plan *MountPlan) (string, error) {
	if hostUser == nil {
		return "", fmt.Errorf("host user is required")
	}

	copyBlock := ""
	if plan != nil && len(plan.CopySpecs) > 0 {
		var builder strings.Builder
		for _, spec := range plan.CopySpecs {
			srcPath := filepath.ToSlash(filepath.Join(SecretsMountPath, spec.SourceRel))
			mode := fmt.Sprintf("%04o", spec.Mode.Perm())
			dest := spec.DestPath
			if spec.IsDir {
				builder.WriteString(fmt.Sprintf("if [ -d %s ]; then\n", shellQuote(srcPath)))
				builder.WriteString(fmt.Sprintf("  mkdir -p %s\n", shellQuote(dest)))
				builder.WriteString(fmt.Sprintf("  if cp -a %s/. %s/ 2>/dev/null; then true; else cp -R %s/. %s/; fi\n", shellQuote(srcPath), shellQuote(dest), shellQuote(srcPath), shellQuote(dest)))
				builder.WriteString(fmt.Sprintf("  chown -R \"$USER_UID\":\"$USER_GID\" %s || true\n", shellQuote(dest)))
				builder.WriteString("fi\n")
				continue
			}
			builder.WriteString(fmt.Sprintf("if [ -f %s ]; then\n", shellQuote(srcPath)))
			builder.WriteString(fmt.Sprintf("  mkdir -p \"$(dirname %s)\"\n", shellQuote(dest)))
			builder.WriteString(fmt.Sprintf("  cp %s %s\n", shellQuote(srcPath), shellQuote(dest)))
			builder.WriteString(fmt.Sprintf("  chmod %s %s\n", mode, shellQuote(dest)))
			builder.WriteString(fmt.Sprintf("  chown \"$USER_UID\":\"$USER_GID\" %s || true\n", shellQuote(dest)))
			builder.WriteString("fi\n")
		}
		builder.WriteString("if [ -d \"$HOME_DIR/.ssh\" ]; then chmod 700 \"$HOME_DIR/.ssh\"; chown \"$USER_UID\":\"$USER_GID\" \"$HOME_DIR/.ssh\" || true; fi\n")
		copyBlock = builder.String()
	}

	agentAuthBlock, err := scripts.RenderAgentAuth(shellQuote(AgentAuthMountDir))
	if err != nil {
		return "", err
	}
	data := scripts.RunnerData{
		UserName:       shellQuote(hostUser.Name),
		UserUID:        hostUser.UID,
		UserGID:        hostUser.GID,
		HomeDir:        shellQuote(GuestHomeForUser(hostUser.Name)),
		ShellPath:      shellQuote(shellPath),
		GenticDir:      shellQuote(GenticMountDir),
		CopyBlock:      copyBlock,
		AgentAuthBlock: agentAuthBlock,
		WorkspacePath: func() string {
			if plan == nil || strings.TrimSpace(plan.WorkspacePath) == "" {
				return ""
			}
			return shellQuote(plan.WorkspacePath)
		}(),
		WorkspaceMount: func() string {
			if plan == nil || strings.TrimSpace(plan.WorkspaceMount) == "" {
				return ""
			}
			return shellQuote(plan.WorkspaceMount)
		}(),
	}
	return scripts.RenderBootstrap(data)
}

func AppendMount(plan *MountPlan, mount ContainerMount) error {
	if plan == nil {
		return fmt.Errorf("mount plan is required")
	}
	if mount.Type == "" {
		mount.Type = MountTypeBind
	}
	if strings.TrimSpace(mount.Source) == "" {
		return fmt.Errorf("mount source is required")
	}
	if strings.TrimSpace(mount.Target) == "" {
		return fmt.Errorf("mount target is required")
	}
	if err := validateContainerPath(mount.Source); err != nil {
		return err
	}
	if err := validateContainerPath(mount.Target); err != nil {
		return err
	}
	for _, existing := range plan.Mounts {
		if existing.Target == mount.Target {
			return nil
		}
	}
	plan.Mounts = append(plan.Mounts, mount)
	return nil
}

func containerMountArg(mount ContainerMount) string {
	mountType := mount.Type
	if mountType == "" {
		mountType = MountTypeBind
	}
	arg := fmt.Sprintf("type=%s,source=%s,target=%s", mountType, mount.Source, mount.Target)
	if mount.ReadOnly {
		arg += ",readonly"
	}
	return arg
}

func shouldRemoveContainers(flags profile.Flags) bool {
	if flags.RemoveContainerOnExit == nil {
		return false
	}
	return *flags.RemoveContainerOnExit
}

func shouldUseSystemd(flags profile.Flags) bool {
	if flags.Systemd == nil {
		return false
	}
	return *flags.Systemd
}

func sanitizeResourceValue(name, value string) (string, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return "", nil
	}
	if strings.ContainsAny(clean, " ,") {
		return "", fmt.Errorf("%s must not contain spaces or commas", name)
	}
	return clean, nil
}

func appendResourceArgs(args []string, cpus, memory string) []string {
	if strings.TrimSpace(cpus) != "" {
		args = append(args, "--cpus", cpus)
	}
	if strings.TrimSpace(memory) != "" {
		args = append(args, "--memory", memory)
	}
	return args
}

func appendCapabilityArgs(args []string, flags profile.Flags) []string {
	for _, capability := range flags.CapAdd {
		if value := strings.TrimSpace(capability); value != "" {
			args = append(args, "--cap-add", value)
		}
	}
	for _, capability := range flags.CapDrop {
		if value := strings.TrimSpace(capability); value != "" {
			args = append(args, "--cap-drop", value)
		}
	}
	return args
}

func appendNetworkingArgs(args []string, flags profile.Flags) []string {
	if network := strings.TrimSpace(flags.Network); network != "" {
		args = append(args, "--network", network)
	}
	if flags.NoDNS != nil && *flags.NoDNS {
		args = append(args, "--no-dns")
	}
	for _, dns := range flags.DNS {
		if value := strings.TrimSpace(dns); value != "" {
			args = append(args, "--dns", value)
		}
	}
	if domain := strings.TrimSpace(flags.DNSDomain); domain != "" {
		args = append(args, "--dns-domain", domain)
	}
	for _, search := range flags.DNSSearch {
		if value := strings.TrimSpace(search); value != "" {
			args = append(args, "--dns-search", value)
		}
	}
	for _, option := range flags.DNSOptions {
		if value := strings.TrimSpace(option); value != "" {
			args = append(args, "--dns-option", value)
		}
	}
	return args
}

func buildRunArgs(opts BuildRunArgsOptions) []string {
	image := opts.Image
	containerName := opts.ContainerName
	profileName := opts.ProfileName
	hostname := opts.Hostname
	flags := opts.Flags
	plan := opts.Plan
	hostUser := opts.HostUser
	desiredWorkdir := opts.DesiredWorkdir
	containerWorkdir := opts.ContainerWorkdir
	detached := opts.Detached
	tty := opts.TTY
	cpuValue := opts.CPU
	memoryValue := opts.Memory
	ulimits := opts.Ulimits

	args := []string{"run"}
	if detached {
		args = append(args, "--progress", "none")
	}
	if detached {
		args = append(args, "-d")
	} else if tty {
		args = append(args, "-it")
	}
	if shouldRemoveContainers(flags) {
		args = append(args, "--rm")
	}
	if strings.TrimSpace(flags.Platform) != "" {
		args = append(args, "--platform", strings.TrimSpace(flags.Platform))
	}
	if !shouldUseSystemd(flags) && flags.Init != nil && *flags.Init {
		args = append(args, "--init")
	}
	if !shouldUseSystemd(flags) && strings.TrimSpace(flags.InitImage) != "" {
		args = append(args, "--init-image", strings.TrimSpace(flags.InitImage))
	}
	args = appendNetworkingArgs(args, flags)
	args = appendResourceArgs(args, cpuValue, memoryValue)
	for _, limit := range ulimits {
		args = append(args, "--ulimit", limit)
	}
	args = appendCapabilityArgs(args, flags)
	args = append(args, "--name", containerName)
	args = append(args, "--uid", "0")
	args = append(args, "--gid", "0")
	args = append(args, "--label", ContainerLabelKey+"=1")
	args = append(args, "--label", LabelProfileKey+"="+profileName)
	args = append(args, "--label", LabelImageKey+"="+image)
	args = append(args, "--label", LabelUserKey+"="+hostUser.Name)
	args = append(args, "--label", LabelHostnameKey+"="+hostname)
	if strings.TrimSpace(desiredWorkdir) != "" {
		args = append(args, "--label", LabelWorkspaceKey+"="+desiredWorkdir)
	}
	if strings.TrimSpace(containerWorkdir) != "" {
		args = append(args, "--workdir", containerWorkdir)
	}
	for _, mount := range plan.Mounts {
		args = append(args, "--mount", containerMountArg(mount))
	}
	env := []string{"NED_HOSTNAME=" + hostname}
	if strings.TrimSpace(flags.GenticDir) != "" {
		env = append(env, "NED_GENTIC_DIR="+strings.TrimSpace(flags.GenticDir))
	}
	if flags.GenticDebug != nil && *flags.GenticDebug {
		env = append(env, "NED_GENTIC_DEBUG=1")
	}
	if opts.AgentAuth && hostUser != nil {
		// Keep Claude's .claude.json beside its credentials on the auth volume.
		env = append(env, "CLAUDE_CONFIG_DIR="+GuestHomeForUser(hostUser.Name)+"/.claude")
	}
	env = append(env, ProxyEnvEntries(flags)...)
	for _, entry := range env {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		args = append(args, "--env", entry)
	}
	if shouldUseSystemd(flags) {
		args = append(args, "--entrypoint", "/sbin/init")
		args = append(args, image)
		return args
	}
	args = append(args, "--entrypoint", "/bin/sh")
	args = append(args, image)
	args = append(args, filepath.Join(RunnerMountDir, RunnerScriptName))
	return args
}

func diskVolumeName(containerName string) (string, error) {
	if strings.TrimSpace(containerName) == "" {
		return "", fmt.Errorf("container name is required")
	}
	sanitized := sanitizeImageComponent(containerName)
	if sanitized == "" {
		return "", fmt.Errorf("invalid container name for disk volume: %s", containerName)
	}
	return fmt.Sprintf("ned-disk-%s", sanitized), nil
}

// AgentAuthVolumeName is the per-VM volume that holds agent CLI logins. It
// lives on the VM's own disk, so logins survive VM recreation and are only
// removed with the VM by `ned remove`.
func AgentAuthVolumeName(containerName string) (string, error) {
	if strings.TrimSpace(containerName) == "" {
		return "", fmt.Errorf("container name is required")
	}
	sanitized := sanitizeImageComponent(containerName)
	if sanitized == "" {
		return "", fmt.Errorf("invalid container name for agent auth volume: %s", containerName)
	}
	return fmt.Sprintf("ned-auth-%s", sanitized), nil
}

// DeleteAgentAuthVolume removes a VM's agent login volume if it exists.
func DeleteAgentAuthVolume(bin, containerName string) error {
	name, err := AgentAuthVolumeName(containerName)
	if err != nil {
		return err
	}
	exists, err := volumeExists(bin, name)
	if err != nil || !exists {
		return err
	}
	out, err := containerCommandOutput(bin, "volume", "delete", name)
	if err != nil {
		return fmt.Errorf("volume delete failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func volumeExists(bin, name string) (bool, error) {
	if strings.TrimSpace(name) == "" {
		return false, fmt.Errorf("volume name is required")
	}
	out, err := containerCommandOutput(bin, "volume", "list", "-q")
	if err != nil {
		return false, fmt.Errorf("volume list failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return false, nil
	}
	for _, line := range strings.Split(trimmed, "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

func getContainerRuntimeRoot(bin string) (string, error) {
	return FetchContainerDataRoot(bin)
}

func ensureVolume(bin, name, size string) error {
	exists, err := volumeExists(bin, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	args := []string{"volume", "create"}
	if strings.TrimSpace(size) != "" {
		args = append(args, "-s", size)
	}
	args = append(args, name)
	if out, err := containerCommandOutput(bin, args...); err != nil {
		return fmt.Errorf("failed to create volume %q: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ensureDiskVolume(bin, containerName, size string) (string, error) {
	if size != "" {
		needed, err := ParseSize(size)
		if err != nil {
			return "", fmt.Errorf("invalid disk size %q: %v", size, err)
		}

		rootDir, err := getContainerRuntimeRoot(bin)
		if err != nil {
			logging.Debugf("Failed to determine container root dir: %v", err)
			rootDir = "" // Fallback handled by CheckDiskSpace
		}

		if err := CheckDiskSpace(rootDir, needed); err != nil {
			return "", fmt.Errorf("disk validation failed: %v", err)
		}
	}

	volumeName, err := diskVolumeName(containerName)
	if err != nil {
		return "", err
	}
	if err := ensureVolume(bin, volumeName, size); err != nil {
		return "", err
	}
	return volumeName, nil
}

func WaitForContainerReady(bin, name string) error {
	return waitForContainerReady(name, containerReadyTimeout, containerReadyPollInterval, func() error {
		cmd := exec.Command(bin, "exec", name, "test", "-f", ContainerReadyFile)
		cmd.Env = os.Environ()
		return cmd.Run()
	}, time.Sleep)
}

func waitForContainerReady(name string, timeout, interval time.Duration, probe func() error, sleep func(time.Duration)) error {
	if timeout <= 0 {
		timeout = containerReadyTimeout
	}
	if interval <= 0 {
		interval = containerReadyPollInterval
	}
	if probe == nil {
		return fmt.Errorf("container readiness probe is required")
	}
	if sleep == nil {
		sleep = time.Sleep
	}
	attempts := readinessAttempts(timeout, interval)
	for attempt := 0; attempt < attempts; attempt++ {
		if err := probe(); err == nil {
			return nil
		}
		if attempt+1 < attempts {
			sleep(interval)
		}
	}
	return fmt.Errorf("timeout waiting up to %s for container %s to be ready", timeout, name)
}

func readinessAttempts(timeout, interval time.Duration) int {
	attempts := int(timeout / interval)
	if timeout%interval != 0 {
		attempts++
	}
	if attempts < 1 {
		attempts = 1
	}
	return attempts
}

const systemdReadyProbe = `if [ ! -x /sbin/init ]; then echo __ned_systemd_missing_init__; exit 41; fi
if ! command -v systemctl >/dev/null 2>&1; then echo __ned_systemd_missing_systemctl__; exit 42; fi
if [ ! -d /run/systemd/system ]; then echo __ned_systemd_booting__; exit 43; fi
state="$(systemctl is-system-running --no-pager 2>/dev/null || true)"
case "$state" in
  running|degraded) exit 0 ;;
  starting|initializing) echo "__ned_systemd_booting__:$state"; exit 44 ;;
  *) echo "__ned_systemd_state__:$state"; exit 45 ;;
esac`

func WaitForContainerSystemdReady(bin, name string) error {
	for i := 0; i < 120; i++ {
		cmd := exec.Command(bin, "exec", name, "/bin/sh", "-lc", systemdReadyProbe)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		trimmed := strings.TrimSpace(string(out))
		if err == nil {
			return nil
		}
		switch {
		case strings.Contains(trimmed, "__ned_systemd_missing_init__"):
			return fmt.Errorf("systemd mode requires /sbin/init in the image")
		case strings.Contains(trimmed, "__ned_systemd_missing_systemctl__"):
			return fmt.Errorf("systemd mode requires an image with systemd installed (missing systemctl)")
		case strings.Contains(trimmed, "__ned_systemd_state__:"):
			state := strings.TrimSpace(strings.TrimPrefix(trimmed, "__ned_systemd_state__:"))
			if state == "" {
				state = "unknown"
			}
			return fmt.Errorf("systemd mode failed: systemctl reported %q", state)
		}
		exists, status, statusErr := ContainerStatusByName(bin, name)
		if statusErr == nil && (!exists || status != "running") {
			return fmt.Errorf("systemd mode failed before readiness: vm %s is %s", name, status)
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for systemd in vm %s; ensure the image boots /sbin/init and includes systemd", name)
}

func BootstrapContainerSystemd(bin, name string) error {
	scriptPath := filepath.Join(RunnerMountDir, BootstrapScriptName)
	cmd := exec.Command(bin, "exec", name, "/bin/sh", scriptPath)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemd bootstrap failed: %v", err)
	}
	return WaitForContainerReady(bin, name)
}

func RunContainer(opts RunContainerOptions) error {
	runnerPath, runnerCleanup, err := prepareRunnerScript(opts.HostUser, opts.ShellPath, opts.DesiredWorkdir, opts.Plan, opts.KeepAlive, shouldUseSystemd(opts.Flags), opts.RunnerDir)
	if err != nil {
		return err
	}
	defer runnerCleanup()

	if err := AppendMount(opts.Plan, ContainerMount{
		Source:   filepath.Dir(runnerPath),
		Target:   RunnerMountDir,
		ReadOnly: true,
	}); err != nil {
		return err
	}

	cpuValue, err := sanitizeResourceValue("cpus", opts.Flags.CPUs)
	if err != nil {
		return err
	}
	memoryValue, err := sanitizeResourceValue("memory", opts.Flags.Memory)
	if err != nil {
		return err
	}
	diskValue, err := sanitizeResourceValue("disk", opts.Flags.Disk)
	if err != nil {
		return err
	}
	ulimits, err := NormalizeUlimits(opts.Flags.Ulimits)
	if err != nil {
		return err
	}
	if diskValue != "" {
		volumeName, err := ensureDiskVolume(opts.Bin, opts.ContainerName, diskValue)
		if err != nil {
			return err
		}
		if err := AppendMount(opts.Plan, ContainerMount{
			Type:     MountTypeVolume,
			Source:   volumeName,
			Target:   DiskMountDir,
			ReadOnly: false,
		}); err != nil {
			return err
		}
	}

	authVolume, err := AgentAuthVolumeName(opts.ContainerName)
	if err != nil {
		return err
	}
	if err := ensureVolume(opts.Bin, authVolume, AgentAuthVolumeSize); err != nil {
		return err
	}
	if err := AppendMount(opts.Plan, ContainerMount{
		Type:   MountTypeVolume,
		Source: authVolume,
		Target: AgentAuthMountDir,
	}); err != nil {
		return err
	}

	args := buildRunArgs(BuildRunArgsOptions{
		Image:            opts.Image,
		ContainerName:    opts.ContainerName,
		ProfileName:      opts.ProfileName,
		Hostname:         opts.Hostname,
		Flags:            opts.Flags,
		Plan:             opts.Plan,
		HostUser:         opts.HostUser,
		DesiredWorkdir:   opts.DesiredWorkdir,
		ContainerWorkdir: opts.ContainerWorkdir,
		Detached:         opts.Detached,
		TTY:              WantsTTY(),
		CPU:              cpuValue,
		Memory:           memoryValue,
		Ulimits:          ulimits,
		AgentAuth:        true,
	})

	cmd := exec.Command(opts.Bin, args...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	if opts.Detached {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err = cmd.Run()
	if opts.Detached {
		if stderrBuf.Len() > 0 {
			output.WriteFormattedLines(output.StderrWriter(), stderrBuf.String())
		}
		if err != nil {
			if stdoutBuf.Len() > 0 {
				output.WriteFormattedLines(output.StdoutWriter(), stdoutBuf.String())
			}
		} else {
			stdout := stdoutBuf.String()
			trimmed := strings.TrimSpace(stdout)
			if trimmed != "" && trimmed != opts.ContainerName {
				output.WriteFormattedLines(output.StdoutWriter(), stdout)
			}
		}
	}
	if err == nil && opts.Detached {
		// Give the container a moment to start and open the runner script before we cleanup
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

func containerExportArgs(id, output string) ([]string, error) {
	containerID := strings.TrimSpace(id)
	if containerID == "" {
		return nil, fmt.Errorf("container ID is required")
	}
	args := []string{"export"}
	if trimmed := strings.TrimSpace(output); trimmed != "" {
		args = append(args, "--output", trimmed)
	}
	args = append(args, containerID)
	return args, nil
}

func ContainerExport(bin, id, output string) error {
	args, err := containerExportArgs(id, output)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func ContainerSystemStatus(bin, format string) ([]byte, error) {
	trimmedFormat := strings.TrimSpace(format)
	if trimmedFormat == "" {
		return nil, fmt.Errorf("format is required")
	}
	out, err := containerCommandOutput(bin, "system", "status", "--format", trimmedFormat)
	if err != nil {
		return out, FormatCommandOutputError(err, out)
	}
	return SanitizeStructuredCommandOutput(out), nil
}

func ContainerImageSave(bin, image, outputPath string) error {
	args := []string{"image", "save", "--output", outputPath, image}
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ContainerImageTag(bin, source, target string) error {
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("source image is required")
	}
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("target image is required")
	}
	args := []string{"image", "tag", source, target}
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ContainerImageDelete(bin, image string) error {
	if strings.TrimSpace(image) == "" {
		return fmt.Errorf("image name is required")
	}
	out, err := containerCommandOutput(bin, "image", "delete", "--force", image)
	if err != nil {
		return fmt.Errorf("image delete failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ContainerImageLoad(bin, inputPath string) error {
	if strings.TrimSpace(inputPath) == "" {
		return fmt.Errorf("input path is required")
	}
	args := []string{"image", "load", "--input", inputPath}
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ContainerLogs(bin, name string) (string, error) {
	out, err := containerCommandOutput(bin, "logs", name)
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %v", err)
	}
	return string(out), nil
}

type ImageListEntry struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	ID         string `json:"id"`
	Created    string `json:"created"`
	Size       string `json:"size"`
}

type containerImageDescriptor struct {
	Digest      string `json:"digest"`
	Size        int64  `json:"size"`
	Annotations struct {
		Created string `json:"org.opencontainers.image.created"`
	} `json:"annotations"`
}

type containerRuntimeImage struct {
	Descriptor containerImageDescriptor `json:"descriptor"`
	Reference  string                   `json:"reference"`
	ID         string                   `json:"id"`
	Variants   []struct {
		Size int64 `json:"size"`
	} `json:"variants"`
	Configuration struct {
		CreationDate string                   `json:"creationDate"`
		Descriptor   containerImageDescriptor `json:"descriptor"`
		Name         string                   `json:"name"`
	} `json:"configuration"`
}

type containerInspectEntry struct {
	Name     string `json:"name"`
	Variants []struct {
		Size int64 `json:"size"`
	} `json:"variants"`
	Configuration struct {
		Name string `json:"name"`
	} `json:"configuration"`
	// For standard docker inspect, size might be at top level or VirtualSize
	Size        int64 `json:"Size"`
	VirtualSize int64 `json:"VirtualSize"`
}

func inspectEntrySize(entry containerInspectEntry) int64 {
	if len(entry.Variants) > 0 {
		return entry.Variants[0].Size
	}
	if entry.Size != 0 {
		return entry.Size
	}
	return entry.VirtualSize
}

func inspectEntryName(entry containerInspectEntry) string {
	if strings.TrimSpace(entry.Name) != "" {
		return entry.Name
	}
	return entry.Configuration.Name
}

func ListImages(bin string) ([]ImageListEntry, error) {
	out, err := containerCommandOutput(bin, "image", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("image list failed: %w", FormatCommandOutputError(err, out))
	}

	out = sanitizeJSONOutput(out)
	output := strings.TrimSpace(string(out))
	if output == "" {
		return []ImageListEntry{}, nil
	}

	rawImages, err := parseContainerRuntimeImages([]byte(output))
	if err != nil {
		return nil, err
	}

	var refs []string
	for i := range rawImages {
		if ref := containerRuntimeImageReference(rawImages[i]); ref != "" {
			ref, err := NormalizeCLIReference("image reference", ref)
			if err != nil {
				return nil, fmt.Errorf("invalid image reference in runtime output: %w", err)
			}
			setContainerRuntimeImageReference(&rawImages[i], ref)
			refs = append(refs, ref)
		}
	}

	sizes := make(map[string]int64)
	if len(refs) > 0 {
		// Bulk inspect
		// We process in chunks to avoid command line length limits if necessary, but for now all at once
		inspectArgs := append([]string{"image", "inspect"}, refs...)
		inspectOut, err := containerCommandOutput(bin, inspectArgs...)
		if err == nil {
			inspectOut = sanitizeJSONOutput(inspectOut)
			var inspectResults []containerInspectEntry
			if err := json.Unmarshal(inspectOut, &inspectResults); err == nil {
				for _, res := range inspectResults {
					if name := inspectEntryName(res); name != "" {
						sizes[name] = inspectEntrySize(res)
					}
				}
			}
		}
	}

	return imageListEntriesFromRuntimeImages(rawImages, sizes), nil
}

func parseContainerRuntimeImages(output []byte) ([]containerRuntimeImage, error) {
	output = sanitizeJSONOutput(output)
	text := strings.TrimSpace(string(output))
	if text == "" {
		return []containerRuntimeImage{}, nil
	}
	var rawImages []containerRuntimeImage
	if err := json.Unmarshal([]byte(text), &rawImages); err == nil {
		return rawImages, nil
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var img containerRuntimeImage
		if err := json.Unmarshal([]byte(line), &img); err != nil {
			return nil, fmt.Errorf("failed to parse image list line: %v", err)
		}
		rawImages = append(rawImages, img)
	}
	return rawImages, nil
}

func imageListEntriesFromRuntimeImages(rawImages []containerRuntimeImage, sizes map[string]int64) []ImageListEntry {
	entries := make([]ImageListEntry, 0, len(rawImages))
	for _, img := range rawImages {
		ref := containerRuntimeImageReference(img)
		repo, tag := splitRuntimeImageReference(ref)
		if repo == "" {
			repo = ref
		}
		if tag == "" {
			tag = "<none>"
		}

		id := shortDigest(containerRuntimeImageID(img))

		created := containerRuntimeImageCreated(img)
		if created == "" {
			created = "N/A"
		} else {
			created = formatImageCreated(created)
		}

		sizeBytes := containerRuntimeImageSize(img)
		if s, ok := sizes[ref]; ok && s > 0 {
			sizeBytes = s
		}

		entries = append(entries, ImageListEntry{
			Repository: repo,
			Tag:        tag,
			ID:         id,
			Created:    created,
			Size:       formatSize(sizeBytes),
		})
	}
	return entries
}

func splitRuntimeImageReference(ref string) (string, string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ""
	}
	if repo, digest, ok := strings.Cut(ref, "@"); ok {
		if digest == "" {
			digest = "<none>"
		}
		return repo, digest
	}
	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")
	if lastColon > lastSlash {
		return ref[:lastColon], ref[lastColon+1:]
	}
	return ref, "<none>"
}

func containerRuntimeImageReference(img containerRuntimeImage) string {
	if strings.TrimSpace(img.Reference) != "" {
		return img.Reference
	}
	return img.Configuration.Name
}

func setContainerRuntimeImageReference(img *containerRuntimeImage, ref string) {
	if strings.TrimSpace(img.Reference) != "" {
		img.Reference = ref
		return
	}
	img.Configuration.Name = ref
}

func containerRuntimeImageDescriptorFor(img containerRuntimeImage) containerImageDescriptor {
	if img.Descriptor.Digest != "" || img.Descriptor.Size != 0 || img.Descriptor.Annotations.Created != "" {
		return img.Descriptor
	}
	return img.Configuration.Descriptor
}

func containerRuntimeImageID(img containerRuntimeImage) string {
	descriptor := containerRuntimeImageDescriptorFor(img)
	if strings.TrimSpace(descriptor.Digest) != "" {
		return descriptor.Digest
	}
	return img.ID
}

func containerRuntimeImageCreated(img containerRuntimeImage) string {
	descriptor := containerRuntimeImageDescriptorFor(img)
	if strings.TrimSpace(descriptor.Annotations.Created) != "" {
		return descriptor.Annotations.Created
	}
	return img.Configuration.CreationDate
}

func containerRuntimeImageSize(img containerRuntimeImage) int64 {
	if len(img.Variants) > 0 && img.Variants[0].Size > 0 {
		return img.Variants[0].Size
	}
	descriptor := containerRuntimeImageDescriptorFor(img)
	return descriptor.Size
}

func formatImageCreated(raw string) string {
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return formatRelativeTime(t)
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return formatRelativeTime(t)
	}
	return raw
}

func formatRelativeTime(t time.Time) string {
	diff := time.Since(t)
	if diff < time.Second {
		return "Just now"
	}
	if diff < time.Minute {
		return fmt.Sprintf("%d seconds ago", int(diff.Seconds()))
	}
	if diff < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(diff.Hours()))
	}
	if diff < 48*time.Hour {
		return "Yesterday"
	}
	if diff < 30*24*time.Hour {
		return fmt.Sprintf("%d days ago", int(diff.Hours()/24))
	}
	if diff < 12*30*24*time.Hour {
		return fmt.Sprintf("%d months ago", int(diff.Hours()/(24*30)))
	}
	return fmt.Sprintf("%d years ago", int(diff.Hours()/(24*365)))
}

func shortDigest(digest string) string {
	if strings.HasPrefix(digest, "sha256:") && len(digest) > 12 {
		return digest[7:19]
	}
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}
