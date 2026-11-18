package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type SystemDiskUsageEntry struct {
	Active           int   `json:"active"`
	Total            int   `json:"total"`
	SizeBytes        int64 `json:"sizeInBytes"`
	ReclaimableBytes int64 `json:"reclaimable"`
}

type SystemDiskUsage struct {
	Containers SystemDiskUsageEntry `json:"containers"`
	Images     SystemDiskUsageEntry `json:"images"`
	Volumes    SystemDiskUsageEntry `json:"volumes"`
}

type PathUsage struct {
	Path          string
	UsedBytes     int64
	ApparentBytes int64
}

type FilesystemUsage struct {
	Filesystem string
	Mount      string
	SizeBytes  int64
	UsedBytes  int64
	FreeBytes  int64
	CheckedAt  time.Time
}

func FetchSystemDiskUsage(bin string) (SystemDiskUsage, error) {
	if strings.TrimSpace(bin) == "" {
		return SystemDiskUsage{}, fmt.Errorf("container runtime path is required")
	}
	out, err := runtimeCommandOutput(bin, "system", "df", "--format", "json")
	if err != nil {
		return SystemDiskUsage{}, fmt.Errorf("container system df failed: %w", FormatCommandOutputError(err, out))
	}
	return parseSystemDiskUsage(out)
}

func FetchContainerDataRoot(bin string) (string, error) {
	if strings.TrimSpace(bin) == "" {
		return "", fmt.Errorf("container runtime path is required")
	}
	out, err := runtimeCommandOutput(bin, "system", "status", "--format", "json")
	if err != nil {
		return "", fmt.Errorf("container system status failed: %w", FormatCommandOutputError(err, out))
	}
	return parseContainerDataRoot(out)
}

func FetchPathUsage(path string) (PathUsage, error) {
	if strings.TrimSpace(path) == "" {
		return PathUsage{}, fmt.Errorf("path is required")
	}
	used, err := duBytes(path)
	if err != nil {
		return PathUsage{}, err
	}
	apparent, err := duApparentBytes(path)
	if err != nil {
		apparent = 0
	}
	return PathUsage{
		Path:          path,
		UsedBytes:     used,
		ApparentBytes: apparent,
	}, nil
}

func FetchFilesystemUsage(path string) (FilesystemUsage, error) {
	if strings.TrimSpace(path) == "" {
		return FilesystemUsage{}, fmt.Errorf("path is required")
	}
	out, err := runtimeCommandOutput("df", "-k", "-P", path)
	if err != nil {
		return FilesystemUsage{}, fmt.Errorf("df failed: %v", err)
	}
	usage, err := parseDFOutput(string(out))
	if err != nil {
		return FilesystemUsage{}, err
	}
	if fs, err := runtimeCommandOutput("stat", "-f", "%T", path); err == nil {
		usage.Filesystem = strings.TrimSpace(string(fs))
	}
	usage.CheckedAt = time.Now().UTC()
	return usage, nil
}

func parseSystemDiskUsage(out []byte) (SystemDiskUsage, error) {
	var usage SystemDiskUsage
	if err := json.Unmarshal(bytes.TrimSpace(sanitizeJSONOutput(out)), &usage); err != nil {
		return SystemDiskUsage{}, fmt.Errorf("failed to parse system df output: %v", err)
	}
	return usage, nil
}

func parseContainerDataRoot(out []byte) (string, error) {
	var payload containerSystemStatusPayload
	if err := json.Unmarshal(bytes.TrimSpace(sanitizeJSONOutput(out)), &payload); err != nil {
		return "", fmt.Errorf("failed to parse container system status: %v", err)
	}
	if payload.Paths == nil {
		return "", fmt.Errorf("container system status did not include paths.appRoot")
	}
	appRoot := strings.TrimSpace(payload.Paths.AppRoot)
	if appRoot == "" {
		return "", fmt.Errorf("container system status paths.appRoot is empty")
	}
	return NormalizeCLIPath("runtime data root", appRoot)
}

func parseDFOutput(out string) (FilesystemUsage, error) {
	lines := strings.Split(SanitizeStructuredCommandText([]byte(out)), "\n")
	if len(lines) < 2 {
		return FilesystemUsage{}, fmt.Errorf("unexpected df output: %q", out)
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 6 {
		return FilesystemUsage{}, fmt.Errorf("unexpected df output: %q", out)
	}
	sizeBlocks, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return FilesystemUsage{}, fmt.Errorf("invalid df size: %v", err)
	}
	usedBlocks, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return FilesystemUsage{}, fmt.Errorf("invalid df used: %v", err)
	}
	freeBlocks, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return FilesystemUsage{}, fmt.Errorf("invalid df free: %v", err)
	}
	mount := strings.Join(fields[5:], " ")
	mount, err = NormalizeCLIPath("filesystem mount", mount)
	if err != nil {
		return FilesystemUsage{}, err
	}
	return FilesystemUsage{
		Mount:     mount,
		SizeBytes: sizeBlocks * 1024,
		UsedBytes: usedBlocks * 1024,
		FreeBytes: freeBlocks * 1024,
	}, nil
}

func duApparentBytes(path string) (int64, error) {
	args := []string{"-sk"}
	args = append(args, "-A")
	args = append(args, path)
	out, err := runtimeCommandOutput("du", args...)
	if err != nil {
		return 0, fmt.Errorf("du failed: %v", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return 0, fmt.Errorf("unexpected du output: %q", out)
	}
	value, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid du size: %v", err)
	}
	return value * 1024, nil
}
