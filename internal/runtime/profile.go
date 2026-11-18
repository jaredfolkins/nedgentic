package runtime

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"nedgentic/internal/logging"
	"nedgentic/internal/profile"
)

func LoadProfile(pathOverride string, require bool, profileName string) (*Profile, error) {
	configPath := strings.TrimSpace(pathOverride)
	usedDefault := false
	if configPath == "" {
		home, err := profile.ResolveHome()
		if err != nil {
			return nil, err
		}
		configPath = profile.ProfileConfigPath(home, profileName)
		usedDefault = true
	}
	if strings.HasPrefix(configPath, "~/") {
		expanded, err := ExpandHome(configPath)
		if err != nil {
			return nil, err
		}
		configPath = expanded
	}

	// Ensure default profile exists
	home, err := profile.ResolveHome()
	if err == nil {
		if _, err := profile.EnsureDefaultProfile(home); err != nil {
			if usedDefault {
				return nil, err
			}
			logging.Warnf("failed to ensure default profile: %v", err)
		}
	} else if usedDefault {
		return nil, err
	}

	file, err := os.Open(configPath)
	if os.IsNotExist(err) {
		if require {
			if err := WriteProfileTemplate(configPath, "", false); err != nil {
				return nil, err
			}
			file, err = os.Open(configPath)
		} else {
			return &Profile{}, nil
		}
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	logging.Debugf("Loading profile from %s", configPath)

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	loaded, err := ParseProfileConfig(content)
	if err != nil {
		if strings.Contains(err.Error(), `unknown field "tools"`) {
			return nil, fmt.Errorf("failed to parse profile %s: imports.tools was removed because copying host agent logins breaks them when tokens rotate; delete the tools entry and sign in inside the VM with ned-agent-login: %w", configPath, err)
		}
		return nil, fmt.Errorf("failed to parse profile %s: %w", configPath, err)
	}
	if loaded.Image != nil {
		logging.Debugf("Loaded image from config: %s", loaded.Image.Current)
	} else {
		logging.Debugf("Loaded image is nil")
	}
	if loaded.Flags == nil {
		loaded.Flags = &profile.Flags{}
	}
	if loaded.Image == nil {
		loaded.Image = &profile.ImageState{Current: profile.DefaultImageReference(), History: map[string]string{}}
	}
	if loaded.Image.History == nil {
		loaded.Image.History = map[string]string{}
	}
	if err := rejectHostAgentAuthImports(&loaded); err != nil {
		return nil, fmt.Errorf("profile %s: %w", configPath, err)
	}
	return &loaded, nil
}

func ParseProfileConfig(content []byte) (Profile, error) {
	var decoded profile.ConfigProfile
	if err := profile.UnmarshalHuJSON(content, &decoded); err != nil {
		return Profile{}, err
	}
	loaded := Profile{
		Shell:     strings.TrimSpace(decoded.Shell),
		Flags:     decoded.Flags,
		Resources: copyResourcesConfig(decoded.Resources),
		Imports:   copyImportsConfig(decoded.Imports),
		Image:     decoded.Image,
	}
	if decoded.Imports != nil {
		loaded.Binaries = trimStringList(decoded.Imports.Binaries)
		loaded.Files = copyFileMounts(decoded.Imports.Files)
		loaded.Directories = copyDirectoryMounts(decoded.Imports.Directories)
	}
	return loaded, nil
}

func trimStringList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func copyFileMounts(values []profile.FileMount) []profile.FileMount {
	if len(values) == 0 {
		return nil
	}
	copied := make([]profile.FileMount, 0, len(values))
	for _, value := range values {
		value.HostPath = strings.TrimSpace(value.HostPath)
		value.VMPath = strings.TrimSpace(value.VMPath)
		value.Mode = strings.TrimSpace(value.Mode)
		if value.HostPath != "" {
			copied = append(copied, value)
		}
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

func copyDirectoryMounts(values []profile.DirectoryMount) []profile.DirectoryMount {
	if len(values) == 0 {
		return nil
	}
	copied := make([]profile.DirectoryMount, 0, len(values))
	for _, value := range values {
		value.HostPath = strings.TrimSpace(value.HostPath)
		value.VMPath = strings.TrimSpace(value.VMPath)
		value.Mode = strings.TrimSpace(value.Mode)
		value.Access = strings.TrimSpace(value.Access)
		if value.HostPath != "" {
			copied = append(copied, value)
		}
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

func copyResourcesConfig(config *profile.ResourcesConfig) *profile.ResourcesConfig {
	if config == nil {
		return nil
	}
	return &profile.ResourcesConfig{
		CPUs:   strings.TrimSpace(config.CPUs),
		Memory: strings.TrimSpace(config.Memory),
		Disk:   strings.TrimSpace(config.Disk),
	}
}

func copyImportsConfig(config *profile.ImportsConfig) *profile.ImportsConfig {
	if config == nil {
		return nil
	}
	return &profile.ImportsConfig{
		Binaries:    trimStringList(config.Binaries),
		Files:       copyFileMounts(config.Files),
		Directories: copyDirectoryMounts(config.Directories),
	}
}

func fileMountExists(mounts []profile.FileMount, candidate profile.FileMount) bool {
	for _, mount := range mounts {
		if strings.EqualFold(strings.TrimSpace(mount.HostPath), strings.TrimSpace(candidate.HostPath)) {
			return true
		}
		if strings.EqualFold(strings.TrimSpace(mount.VMPath), strings.TrimSpace(candidate.VMPath)) {
			return true
		}
	}
	return false
}

func applyProfileShellFiles(files []profile.FileMount, profileDir, shell string, force, replace bool) ([]profile.FileMount, error) {
	if err := profile.EnsureProfileShellFiles(profileDir, shell, force); err != nil {
		return nil, err
	}
	home, err := profile.ResolveHome()
	if err != nil {
		home = ""
	}
	filtered := files
	if replace {
		filtered = files[:0]
		for _, mount := range files {
			vmPath := strings.TrimSpace(mount.VMPath)
			switch vmPath {
			case ".zshrc", ".zprofile", ".bashrc", ".bash_profile":
				continue
			default:
				filtered = append(filtered, mount)
			}
		}
	}
	for _, mount := range profile.ShellFileMountsForHome(profileDir, shell, home) {
		if !fileMountExists(filtered, mount) {
			filtered = append(filtered, mount)
		}
	}
	return filtered, nil
}

func EffectiveFlags(prof *Profile) profile.Flags {
	flags := profile.DefaultFlags()
	if prof == nil {
		return flags
	}
	if prof.Resources != nil && !prof.Resources.IsZero() {
		flags = profile.MergeFlags(flags, prof.Resources.Flags())
	}
	if prof.Flags != nil && !profile.FlagsEmpty(*prof.Flags) {
		flags = profile.MergeFlags(flags, *prof.Flags)
		return flags
	}
	return flags
}

func ExpandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		return filepath.Join(usr.HomeDir, path[2:]), nil
	}
	return path, nil
}

func ResolveShellPath(profile *Profile) (string, error) {
	shell := ""
	if profile != nil {
		shell = strings.TrimSpace(profile.Shell)
	}
	if shell == "" {
		if envShell := strings.TrimSpace(os.Getenv("SHELL")); envShell != "" {
			shell = envShell
		}
	}
	if shell == "" {
		shell = DefaultShellPath
	}
	if !filepath.IsAbs(shell) {
		return "", fmt.Errorf("shell path must be absolute: %s", shell)
	}
	return shell, nil
}

func WriteProfileTemplate(path, image string, force bool) error {
	if path == "" {
		return fmt.Errorf("profile path is required")
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("profile already exists: %s", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	flags := profile.DefaultFlags()
	imageState := &profile.ImageState{Current: profile.DefaultImageReference(), History: map[string]string{}}
	if strings.TrimSpace(image) != "" {
		imageState.Current = strings.TrimSpace(image)
	}
	shell := profile.DefaultShellPath
	dirs := profile.DefaultDirectories()
	files, err := applyProfileShellFiles(profile.DefaultFiles(), filepath.Dir(path), shell, force, true)
	if err != nil {
		return err
	}
	content := profile.TemplateWithShell(profile.TemplateOptions{
		Shell:       shell,
		Flags:       flags,
		Resources:   profile.ResourcesFromFlags(flags),
		Binaries:    nil,
		Files:       files,
		Directories: dirs,
		Image:       imageState,
	})
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return err
	}
	// Build files generation removed
	return nil
}

func WriteProfileConfig(path string, prof *Profile) error {
	if path == "" {
		return fmt.Errorf("profile path is required")
	}
	shell := profile.DefaultShellPath
	if prof != nil {
		if value := strings.TrimSpace(prof.Shell); value != "" {
			shell = value
		}
	}
	flags := profile.DefaultFlags()
	if prof != nil {
		flags = EffectiveFlags(prof)
	}
	files := profile.DefaultFiles()
	dirs := profile.DefaultDirectories()
	binaries := []string(nil)
	image := &profile.ImageState{Current: profile.DefaultImageReference(), History: map[string]string{}}
	if prof != nil && prof.Image != nil {
		image = prof.Image
	}
	if prof != nil && len(prof.Files) > 0 {
		files = prof.Files
	}
	files, err := applyProfileShellFiles(files, filepath.Dir(path), shell, false, false)
	if err != nil {
		return err
	}
	if prof != nil && len(prof.Directories) > 0 {
		dirs = prof.Directories
	}
	if prof != nil && len(prof.Binaries) > 0 {
		binaries = prof.Binaries
	}
	content := profile.TemplateWithShell(profile.TemplateOptions{
		Shell:       shell,
		Flags:       flags,
		Resources:   profile.ResourcesFromFlags(flags),
		Binaries:    binaries,
		Files:       files,
		Directories: dirs,
		Image:       image,
	})
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return err
	}
	return nil
}

func sanitizeImageComponent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	result := strings.Trim(b.String(), "-")
	return result
}

func ContainerNameForProfile(profileName string) (string, error) {
	profilePart := sanitizeImageComponent(profileName)
	if profilePart == "" {
		return "", fmt.Errorf("invalid profile name: %s", profileName)
	}
	return profilePart, nil
}

func ResolveRunImage(profile *Profile) (string, error) {
	if profile == nil || profile.Image == nil {
		return "", fmt.Errorf("profile image is required")
	}
	if value := strings.TrimSpace(profile.Image.Current); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("profile image is required")
}
