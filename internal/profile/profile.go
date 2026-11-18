package profile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"nedgentic/internal/config"
	"nedgentic/internal/release"
)

const (
	BaseDirName        = ".ned"
	ProfilesDirName    = "profiles"
	PersonalDirName    = "personal"
	SessionsDirName    = "sessions"
	StagingDirName     = "staging"
	DependenciesName   = "dependencies.json"
	DefaultProfileName = "default"
	ProfileConfigName  = "config.hujson"
	DefaultShellPath   = "/bin/bash"
)

// DefaultImageReference is the image new profiles start from: the minimal
// image tagged for this ned release (ned-minimal:<version>), or
// ned-minimal:dev for dev builds.
func DefaultImageReference() string {
	return release.DefaultImageRef()
}

func BaseDir(home string) string {
	return filepath.Join(home, BaseDirName)
}

func ProfilesDir(home string) string {
	return filepath.Join(BaseDir(home), ProfilesDirName)
}

func StagingDir(home string) string {
	return filepath.Join(BaseDir(home), StagingDirName)
}

func SessionsDir(home string) string {
	return filepath.Join(BaseDir(home), SessionsDirName)
}

func StagingVMDir(home, name string) (string, error) {
	original := name
	name, err := normalizeVMName(name)
	if err != nil {
		return "", fmt.Errorf("vm name %q is invalid for staging: %v", original, err)
	}
	return filepath.Join(StagingDir(home), name), nil
}

func DependenciesPath(home string) string {
	return filepath.Join(BaseDir(home), DependenciesName)
}

func PersonalProfilesDir(home string) string {
	return filepath.Join(ProfilesDir(home), PersonalDirName)
}

func normalizeVMName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("vm name is required")
	}
	if name == "." || name == ".." || strings.Contains(name, string(os.PathSeparator)) || strings.Contains(name, "/") {
		return "", fmt.Errorf("vm name contains invalid characters")
	}
	return name, nil
}

func ProfileDir(home, name string) string {
	return filepath.Join(PersonalProfilesDir(home), name)
}

func ProfileConfigPath(home, name string) string {
	return filepath.Join(ProfileDir(home, name), ProfileConfigName)
}

func DefaultProfilePath(home string) string {
	return ProfileConfigPath(home, DefaultProfileName)
}

func formatFlagsBlock(flags Flags) string {
	defaults := DefaultFlags()
	entries := []string{}
	addString := func(name, value, defaultValue string) {
		value = strings.TrimSpace(value)
		if value != "" && value != strings.TrimSpace(defaultValue) {
			entries = append(entries, fmt.Sprintf("    %q: %q", name, value))
		}
	}
	addBool := func(name string, value, defaultValue *bool) {
		if value == nil {
			return
		}
		if defaultValue == nil || *value != *defaultValue {
			entries = append(entries, fmt.Sprintf("    %q: %t", name, *value))
		}
	}
	addStringSlice := func(name string, values []string) {
		values = cleanTemplateStringList(values)
		if len(values) == 0 {
			return
		}
		lines := make([]string, 0, len(values)+2)
		lines = append(lines, fmt.Sprintf("    %q: [", name))
		for i, value := range values {
			suffix := ""
			if i < len(values)-1 {
				suffix = ","
			}
			lines = append(lines, fmt.Sprintf("      %q%s", value, suffix))
		}
		lines = append(lines, "    ]")
		entries = append(entries, strings.Join(lines, "\n"))
	}

	addString("log-level", flags.LogLevel, defaults.LogLevel)
	addString("container-bin", flags.ContainerBin, defaults.ContainerBin)
	addBool("init", flags.Init, defaults.Init)
	addString("init-image", flags.InitImage, defaults.InitImage)
	addBool("systemd", flags.Systemd, defaults.Systemd)
	addString("network", flags.Network, defaults.Network)
	addString("proxy", flags.Proxy, defaults.Proxy)
	addString("http-proxy", flags.HTTPProxy, defaults.HTTPProxy)
	addString("https-proxy", flags.HTTPSProxy, defaults.HTTPSProxy)
	addStringSlice("no-proxy", flags.NoProxy)
	addBool("no-dns", flags.NoDNS, defaults.NoDNS)
	addStringSlice("dns", flags.DNS)
	addString("dns-domain", flags.DNSDomain, defaults.DNSDomain)
	addStringSlice("dns-search", flags.DNSSearch)
	addStringSlice("dns-option", flags.DNSOptions)
	addBool("rm-container-on-exit", flags.RemoveContainerOnExit, defaults.RemoveContainerOnExit)
	addString("platform", flags.Platform, defaults.Platform)
	addStringSlice("ulimits", flags.Ulimits)
	addStringSlice("cap-add", flags.CapAdd)
	addStringSlice("cap-drop", flags.CapDrop)

	if len(entries) == 0 {
		return ""
	}
	return "  \"flags\": {\n" + strings.Join(entries, ",\n") + "\n  }"
}

func cleanTemplateStringList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func formatResourcesBlock(resources *ResourcesConfig) string {
	if resources == nil || resources.IsZero() {
		return ""
	}
	entries := []string{}
	if cpus := strings.TrimSpace(resources.CPUs); cpus != "" {
		entries = append(entries, fmt.Sprintf("    %q: %q", "cpus", cpus))
	}
	if memory := strings.TrimSpace(resources.Memory); memory != "" {
		entries = append(entries, fmt.Sprintf("    %q: %q", "memory", memory))
	}
	if disk := strings.TrimSpace(resources.Disk); disk != "" {
		entries = append(entries, fmt.Sprintf("    %q: %q", "disk", disk))
	}
	return "  \"resources\": {\n" + strings.Join(entries, ",\n") + "\n  }"
}

type FileMount struct {
	HostPath string `json:"host"`
	VMPath   string `json:"vm"`
	Mode     string `json:"mode"`
}

type DirectoryMount struct {
	HostPath string `json:"host"`
	VMPath   string `json:"vm"`
	Mode     string `json:"mode"`
	Access   string `json:"access"`
}

type ImageState struct {
	Current string            `json:"current"`
	History map[string]string `json:"history"`
}

func (image *ImageState) UnmarshalJSON(data []byte) error {
	var ref string
	if err := json.Unmarshal(data, &ref); err == nil {
		*image = ImageState{
			Current: strings.TrimSpace(ref),
			History: map[string]string{},
		}
		return nil
	}
	type imageState ImageState
	var decoded imageState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if err := ensureDecoderEOF(decoder); err != nil {
		return err
	}
	if decoded.History == nil {
		decoded.History = map[string]string{}
	}
	*image = ImageState(decoded)
	return nil
}

func formatImageBlock(image *ImageState) string {
	if image == nil || strings.TrimSpace(image.Current) == "" {
		return ""
	}
	if len(image.History) == 0 {
		return fmt.Sprintf("  %q: %q", "image", strings.TrimSpace(image.Current))
	}
	var builder strings.Builder
	builder.WriteString("  \"image\": {\n")
	builder.WriteString(fmt.Sprintf("    %q: %q,\n", "current", strings.TrimSpace(image.Current)))
	builder.WriteString(fmt.Sprintf("    %q: {\n", "history"))
	keys := make([]string, 0, len(image.History))
	for key := range image.History {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		builder.WriteString(fmt.Sprintf("      %q: %q", key, image.History[key]))
		if i < len(keys)-1 {
			builder.WriteString(",")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("    }\n")
	builder.WriteString("  }")
	return builder.String()
}

func formatFilesBlock(files []FileMount) string {
	if len(files) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("  \"files\": [\n")
	for i, mount := range files {
		entries := []string{fmt.Sprintf("      %q: %q", "host", mount.HostPath)}
		if mount.VMPath != "" {
			entries = append(entries, fmt.Sprintf("      %q: %q", "vm", mount.VMPath))
		}
		if mount.Mode != "" {
			entries = append(entries, fmt.Sprintf("      %q: %q", "mode", mount.Mode))
		}
		builder.WriteString("    {\n")
		builder.WriteString(strings.Join(entries, ",\n"))
		builder.WriteString("\n")
		builder.WriteString("    }")
		if i < len(files)-1 {
			builder.WriteString(",")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("  ]\n")
	return builder.String()
}

func formatBinariesBlock(binaries []string) string {
	binaries = cleanTemplateStringList(binaries)
	if len(binaries) == 0 {
		return ""
	}
	return formatStringArrayBlock("binaries", binaries)
}

func formatImportsBlock(config *ImportsConfig) string {
	if config == nil || config.IsZero() {
		return ""
	}
	entries := []string{}
	if block := strings.TrimRight(indentBlock(formatBinariesBlock(config.Binaries), "  "), "\n"); strings.TrimSpace(block) != "" {
		entries = append(entries, block)
	}
	if block := strings.TrimRight(indentBlock(formatFilesBlock(config.Files), "  "), "\n"); strings.TrimSpace(block) != "" {
		entries = append(entries, block)
	}
	if block := strings.TrimRight(indentBlock(formatDirectoriesBlock(config.Directories), "  "), "\n"); strings.TrimSpace(block) != "" {
		entries = append(entries, block)
	}
	if len(entries) == 0 {
		return ""
	}
	return "  \"imports\": {\n" + strings.Join(entries, ",\n") + "\n  }"
}

func formatStringArrayBlock(name string, values []string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("  %q: [\n", name))
	for i, value := range values {
		builder.WriteString(fmt.Sprintf("    %q", value))
		if i < len(values)-1 {
			builder.WriteString(",")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("  ]")
	return builder.String()
}

func formatDirectoriesBlock(dirs []DirectoryMount) string {
	if len(dirs) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("  \"directories\": [\n")
	for i, mount := range dirs {
		entries := []string{fmt.Sprintf("      %q: %q", "host", mount.HostPath)}
		if mount.VMPath != "" {
			entries = append(entries, fmt.Sprintf("      %q: %q", "vm", mount.VMPath))
		}
		mode := strings.TrimSpace(mount.Mode)
		if mode != "" {
			entries = append(entries, fmt.Sprintf("      %q: %q", "mode", mode))
		}
		if !strings.EqualFold(mode, config.MountModeCopy) {
			access := strings.TrimSpace(mount.Access)
			if access == "" {
				access = config.AccessReadWrite
			}
			entries = append(entries, fmt.Sprintf("      %q: %q", "access", access))
		}
		builder.WriteString("    {\n")
		builder.WriteString(strings.Join(entries, ",\n"))
		builder.WriteString("\n")
		builder.WriteString("    }")
		if i < len(dirs)-1 {
			builder.WriteString(",")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("  ]\n")
	return builder.String()
}

func indentBlock(block, indent string) string {
	if strings.TrimSpace(block) == "" {
		return ""
	}
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func TemplateWithShell(opts TemplateOptions) string {
	if strings.TrimSpace(opts.Shell) == "" {
		opts.Shell = DefaultShellPath
	}
	if opts.Image == nil {
		opts.Image = &ImageState{Current: DefaultImageReference(), History: map[string]string{}}
	}
	imports := opts.Imports
	if imports == nil {
		imports = ImportsFromFields(opts.Binaries, opts.Files, opts.Directories)
	}
	resources := opts.Resources
	if resources == nil {
		resources = ResourcesFromFlags(opts.Flags)
	}
	flags := opts.Flags
	flags.CPUs = ""
	flags.Memory = ""
	flags.Disk = ""
	sections := []profileTemplateSection{
		{Comment: "Image selection.", Body: formatImageBlock(opts.Image)},
		{Comment: "Resource sizing defaults.", Body: formatResourcesBlock(resources)},
		{Comment: "Imports from the host.", Body: formatImportsBlock(imports)},
		{Comment: "Advanced runtime flag overrides.", Body: formatFlagsBlock(flags)},
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("{\n  %q: %q", "shell", opts.Shell))
	for _, section := range sections {
		body := strings.TrimRight(section.Body, "\n")
		if strings.TrimSpace(body) == "" {
			continue
		}
		builder.WriteString(",\n")
		if strings.TrimSpace(section.Comment) != "" {
			builder.WriteString("  // " + section.Comment + "\n")
		}
		builder.WriteString(body)
	}
	builder.WriteString("\n}\n")
	return builder.String()
}

type profileTemplateSection struct {
	Comment string
	Body    string
}

func Template() string {
	flags := DefaultFlags()
	image := &ImageState{
		Current: DefaultImageReference(),
		History: map[string]string{},
	}
	return TemplateWithShell(TemplateOptions{
		Shell:       DefaultShellPath,
		Flags:       flags,
		Binaries:    nil,
		Files:       DefaultFiles(),
		Directories: DefaultDirectories(),
		Image:       image,
	})
}

func DefaultFiles() []FileMount {
	return []FileMount{}
}

func DefaultDirectories() []DirectoryMount {
	return []DirectoryMount{}
}

func ResolveHome() (string, error) {
	if sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER")); sudoUser != "" {
		usr, err := user.Lookup(sudoUser)
		if err != nil {
			return "", fmt.Errorf("failed to resolve SUDO_USER %q: %v", sudoUser, err)
		}
		if usr.HomeDir != "" {
			return usr.HomeDir, nil
		}
	}
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	if usr.HomeDir == "" {
		return "", fmt.Errorf("user home directory is empty")
	}
	return usr.HomeDir, nil
}

func EnsureProfileLayout(home string) error {
	if strings.TrimSpace(home) == "" {
		return fmt.Errorf("home directory is required")
	}
	baseDir := ProfilesDir(home)
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return err
	}
	return os.MkdirAll(PersonalProfilesDir(home), 0700)
}

func EnsureDefaultProfile(home string) (string, error) {
	if strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("home directory is required")
	}

	if err := EnsureProfileLayout(home); err != nil {
		return "", err
	}

	path := DefaultProfilePath(home)
	dir := filepath.Dir(path)

	if _, err := os.Stat(path); err == nil {
		// Ensure ownership fixup if it exists
		if err := ensureOwnership(home, dir, path); err != nil {
			return "", err
		}
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	if err := EnsureProfileShellFiles(dir, DefaultShellPath, false); err != nil {
		return "", err
	}
	files := DefaultFiles()
	files = append(files, ShellFileMountsForHome(dir, DefaultShellPath, home)...)
	image := &ImageState{
		Current: DefaultImageReference(),
		History: map[string]string{},
	}
	content := TemplateWithShell(TemplateOptions{
		Shell:       DefaultShellPath,
		Flags:       DefaultFlags(),
		Binaries:    nil,
		Files:       files,
		Directories: DefaultDirectories(),
		Image:       image,
	})
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return "", err
	}
	if err := ensureOwnership(home, dir, path); err != nil {
		return "", err
	}
	return path, nil
}

func ensureOwnership(home, profilesDir, profilePath string) error {
	uid, gid, ok, err := resolveOwner()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	baseDir := filepath.Join(home, BaseDirName)
	profileDir := filepath.Dir(profilePath)
	for _, path := range []string{baseDir, profilesDir, profileDir, profilePath} {
		if err := os.Chown(path, uid, gid); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func resolveOwner() (int, int, bool, error) {
	if os.Geteuid() != 0 {
		return 0, 0, false, nil
	}
	sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER"))
	if sudoUser == "" {
		return 0, 0, false, nil
	}
	usr, err := user.Lookup(sudoUser)
	if err != nil {
		return 0, 0, false, fmt.Errorf("failed to resolve SUDO_USER %q: %v", sudoUser, err)
	}
	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return 0, 0, false, fmt.Errorf("failed to parse uid for %q: %v", sudoUser, err)
	}
	gid, err := strconv.Atoi(usr.Gid)
	if err != nil {
		return 0, 0, false, fmt.Errorf("failed to parse gid for %q: %v", sudoUser, err)
	}
	return uid, gid, true, nil
}
