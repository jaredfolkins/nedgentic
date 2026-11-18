package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

type VolumeEntry struct {
	Name      string
	SizeBytes int64
	UsedBytes int64
	Source    string
}

type volumeListEntry struct {
	Name string `json:"name"`
}

type volumeInspectEntry struct {
	Name        string `json:"name"`
	SizeInBytes int64  `json:"sizeInBytes"`
	Source      string `json:"source"`
}

func ListVolumes(bin string) ([]VolumeEntry, error) {
	out, err := containerCommandOutput(bin, "volume", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("volume list failed: %w", FormatCommandOutputError(err, out))
	}
	out = sanitizeJSONOutput(out)
	var list []volumeListEntry
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("failed to parse volume list: %v", err)
	}
	if len(list) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(list))
	for _, entry := range list {
		if name := strings.TrimSpace(entry.Name); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, nil
	}
	inspectArgs := append([]string{"volume", "inspect"}, names...)
	inspectOut, err := containerCommandOutput(bin, inspectArgs...)
	if err != nil {
		return nil, fmt.Errorf("volume inspect failed: %w", FormatCommandOutputError(err, inspectOut))
	}
	inspectOut = sanitizeJSONOutput(inspectOut)
	var inspected []volumeInspectEntry
	if err := json.Unmarshal(inspectOut, &inspected); err != nil {
		return nil, fmt.Errorf("failed to parse volume inspect: %v", err)
	}
	byName := make(map[string]volumeInspectEntry, len(inspected))
	for _, entry := range inspected {
		byName[strings.TrimSpace(entry.Name)] = entry
	}
	entries := make([]VolumeEntry, 0, len(names))
	for _, name := range names {
		safeName, err := NormalizeCLIIdentifier("volume name", name)
		if err != nil {
			return nil, fmt.Errorf("invalid volume name in runtime output: %w", err)
		}
		info := byName[name]
		source := ""
		if strings.TrimSpace(info.Source) != "" {
			source, err = NormalizeCLIPath("volume source", info.Source)
			if err != nil {
				return nil, fmt.Errorf("invalid volume source in runtime output: %w", err)
			}
		}
		used := int64(-1)
		if source != "" {
			if size, err := duBytes(source); err == nil {
				used = size
			}
		}
		size := info.SizeInBytes
		if size == 0 {
			size = -1
		}
		entries = append(entries, VolumeEntry{
			Name:      safeName,
			SizeBytes: size,
			UsedBytes: used,
			Source:    source,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func duBytes(path string) (int64, error) {
	if strings.TrimSpace(path) == "" {
		return 0, fmt.Errorf("path is required")
	}
	cmd := exec.Command("du", "-sk", path)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return 0, fmt.Errorf("unexpected du output: %s", strings.TrimSpace(string(out)))
	}
	value, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, err
	}
	return value * 1024, nil
}
