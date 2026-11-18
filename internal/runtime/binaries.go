package runtime

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"nedgentic/internal/config"
	"nedgentic/internal/profile"
)

// ApplyHostBinaries adds the profile's "binaries" as file copies into
// /usr/local/bin.
func ApplyHostBinaries(prof *Profile) error {
	if prof == nil || len(prof.Binaries) == 0 {
		return nil
	}
	if prof.Files == nil {
		prof.Files = []profile.FileMount{}
	}
	seen := map[string]struct{}{}
	for _, spec := range prof.Binaries {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		if _, ok := seen[spec]; ok {
			continue
		}
		seen[spec] = struct{}{}
		mount, err := binaryMountFromSpec(spec)
		if err != nil {
			return err
		}
		if mount == nil {
			continue
		}
		if !fileMountExists(prof.Files, *mount) {
			prof.Files = append(prof.Files, *mount)
		}
	}
	return nil
}

func binaryMountFromSpec(spec string) (*profile.FileMount, error) {
	hostPath, name, err := resolveBinarySpec(spec)
	if err != nil {
		return nil, err
	}
	if hostPath == "" || name == "" {
		return nil, nil
	}
	if err := validateHostBinary(hostPath); err != nil {
		return nil, err
	}
	return &profile.FileMount{
		HostPath: hostPath,
		VMPath:   filepath.Join("/usr/local/bin", name),
		Mode:     config.MountModeCopy,
	}, nil
}

func resolveBinarySpec(spec string) (string, string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "", nil
	}
	var hostPath string
	if strings.Contains(spec, string(filepath.Separator)) || strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "~") {
		expanded, err := ExpandHome(spec)
		if err != nil {
			return "", "", err
		}
		if !filepath.IsAbs(expanded) {
			abs, err := filepath.Abs(expanded)
			if err != nil {
				return "", "", err
			}
			expanded = abs
		}
		hostPath = expanded
	} else {
		found, err := exec.LookPath(spec)
		if err != nil {
			return "", "", fmt.Errorf("host binary %q not found in PATH", spec)
		}
		hostPath = found
	}
	info, err := os.Stat(hostPath)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("host binary %q is a directory", hostPath)
	}
	return hostPath, filepath.Base(hostPath), nil
}

func validateHostBinary(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	header := make([]byte, 4)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return err
	}
	header = header[:n]
	if isScriptHeader(header) || isELFHeader(header) {
		return nil
	}
	if isMachOHeader(header) {
		return fmt.Errorf("host binary %q appears to be Mach-O; only Linux binaries or scripts can run in the container", path)
	}
	return fmt.Errorf("host binary %q is not a Linux ELF or script", path)
}

func isScriptHeader(header []byte) bool {
	return len(header) >= 2 && header[0] == '#' && header[1] == '!'
}

func isELFHeader(header []byte) bool {
	return len(header) >= 4 && header[0] == 0x7f && header[1] == 'E' && header[2] == 'L' && header[3] == 'F'
}

func isMachOHeader(header []byte) bool {
	if len(header) < 4 {
		return false
	}
	switch string(header[:4]) {
	case "\xfe\xed\xfa\xce", "\xfe\xed\xfa\xcf", "\xcf\xfa\xed\xfe", "\xce\xfa\xed\xfe", "\xca\xfe\xba\xbe", "\xbe\xba\xfe\xca":
		return true
	}
	return false
}
