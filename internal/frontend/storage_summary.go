package frontend

import (
	"fmt"
	"io"
	"time"

	"nedgentic/internal/backend/runtime"
)

type storageUsageInfo struct {
	Active           int
	Total            int
	SizeBytes        int64
	Size             string
	ReclaimableBytes int64
	Reclaimable      string
}

type storageFilesystemInfo struct {
	Filesystem string
	Mount      string
	SizeBytes  int64
	Size       string
	UsedBytes  int64
	Used       string
	FreeBytes  int64
	Free       string
	CheckedAt  string
}

type storageSummary struct {
	Images                *storageUsageInfo
	Containers            *storageUsageInfo
	Volumes               *storageUsageInfo
	SystemError           string
	RuntimeDataRoot       string
	RuntimeDataRootError  string
	RuntimeDataUsedBytes  *int64
	RuntimeDataUsed       string
	RuntimeDataError      string
	OtherRuntimeDataBytes *int64
	OtherRuntimeData      string
	Filesystem            *storageFilesystemInfo
	FilesystemError       string
}

func collectStorageSummary(bin string) storageSummary {
	summary := storageSummary{}
	usage, usageErr := runtime.FetchSystemDiskUsage(bin)
	if usageErr != nil {
		summary.SystemError = usageErr.Error()
	} else {
		summary.Images = newStorageUsageInfo(usage.Images)
		summary.Containers = newStorageUsageInfo(usage.Containers)
		summary.Volumes = newStorageUsageInfo(usage.Volumes)
	}

	dataRoot, dataRootErr := runtime.FetchContainerDataRoot(bin)
	if dataRootErr != nil {
		summary.RuntimeDataRootError = dataRootErr.Error()
		return summary
	}
	summary.RuntimeDataRoot = dataRoot

	pathUsage, pathErr := runtime.FetchPathUsage(dataRoot)
	if pathErr != nil {
		summary.RuntimeDataError = pathErr.Error()
	} else {
		value := pathUsage.UsedBytes
		summary.RuntimeDataUsedBytes = &value
		summary.RuntimeDataUsed = runtime.FormatBytes(pathUsage.UsedBytes)
		if usageErr == nil {
			other := max(pathUsage.UsedBytes-(usage.Images.SizeBytes+usage.Containers.SizeBytes+usage.Volumes.SizeBytes), 0)
			summary.OtherRuntimeDataBytes = &other
			summary.OtherRuntimeData = runtime.FormatBytes(other)
		}
	}

	fsUsage, fsErr := runtime.FetchFilesystemUsage(dataRoot)
	if fsErr != nil {
		summary.FilesystemError = fsErr.Error()
		return summary
	}
	summary.Filesystem = &storageFilesystemInfo{
		Filesystem: fsUsage.Filesystem,
		Mount:      fsUsage.Mount,
		SizeBytes:  fsUsage.SizeBytes,
		Size:       runtime.FormatBytes(fsUsage.SizeBytes),
		UsedBytes:  fsUsage.UsedBytes,
		Used:       runtime.FormatBytes(fsUsage.UsedBytes),
		FreeBytes:  fsUsage.FreeBytes,
		Free:       runtime.FormatBytes(fsUsage.FreeBytes),
		CheckedAt:  fsUsage.CheckedAt.UTC().Format(time.RFC3339),
	}
	return summary
}

func writeStorageSummaryText(writer io.Writer, summary storageSummary) error {
	if _, err := fmt.Fprintln(writer, "Storage summary:"); err != nil {
		return err
	}
	if summary.SystemError != "" {
		if _, err := fmt.Fprintf(writer, "  - System usage unavailable: %s\n", summary.SystemError); err != nil {
			return err
		}
	} else {
		if err := writeStorageUsageLine(writer, "Images (shared-layer aware)", summary.Images); err != nil {
			return err
		}
		if err := writeStorageUsageLine(writer, "Containers", summary.Containers); err != nil {
			return err
		}
		if err := writeStorageUsageLine(writer, "Volumes", summary.Volumes); err != nil {
			return err
		}
	}

	if summary.RuntimeDataRootError != "" {
		_, err := fmt.Fprintf(writer, "  - Runtime data root unavailable: %s\n", summary.RuntimeDataRootError)
		return err
	}
	if _, err := fmt.Fprintf(writer, "  - Runtime data root: %s\n", summary.RuntimeDataRoot); err != nil {
		return err
	}
	if summary.RuntimeDataError != "" {
		if _, err := fmt.Fprintf(writer, "  - Runtime data size unavailable: %s\n", summary.RuntimeDataError); err != nil {
			return err
		}
	} else if summary.RuntimeDataUsedBytes != nil {
		if _, err := fmt.Fprintf(writer, "  - Runtime data on disk: %s\n", summary.RuntimeDataUsed); err != nil {
			return err
		}
		if summary.OtherRuntimeDataBytes != nil {
			if _, err := fmt.Fprintf(writer, "  - Other runtime data (build cache, logs, rootfs): %s\n", summary.OtherRuntimeData); err != nil {
				return err
			}
		}
	}

	if summary.FilesystemError != "" {
		_, err := fmt.Fprintf(writer, "  - Filesystem usage unavailable: %s\n", summary.FilesystemError)
		return err
	}
	if summary.Filesystem != nil {
		_, err := fmt.Fprintf(writer, "  - Filesystem (%s) free %s used %s (checked %s)\n",
			valueOrDefault(summary.Filesystem.Filesystem, "unknown"),
			summary.Filesystem.Free,
			summary.Filesystem.Used,
			summary.Filesystem.CheckedAt,
		)
		return err
	}
	return nil
}

func newStorageUsageInfo(entry runtime.SystemDiskUsageEntry) *storageUsageInfo {
	return &storageUsageInfo{
		Active:           entry.Active,
		Total:            entry.Total,
		SizeBytes:        entry.SizeBytes,
		Size:             runtime.FormatBytes(entry.SizeBytes),
		ReclaimableBytes: entry.ReclaimableBytes,
		Reclaimable:      runtime.FormatBytes(entry.ReclaimableBytes),
	}
}

func writeStorageUsageLine(writer io.Writer, label string, value *storageUsageInfo) error {
	if value == nil {
		return nil
	}
	_, err := fmt.Fprintf(writer, "  - %s: %s total, %s reclaimable (%d active of %d)\n",
		label,
		value.Size,
		value.Reclaimable,
		value.Active,
		value.Total,
	)
	return err
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
