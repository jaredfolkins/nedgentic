package frontend

import (
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

const nedDiskVolumePrefix = "ned-disk-"

var storageAll bool

var storageTableFields = []tableField[runtime.VolumeEntry]{
	{Header: "NAME", Value: func(item runtime.VolumeEntry) string { return item.Name }},
	{Header: "SIZE", Value: func(item runtime.VolumeEntry) string { return volumeSizeLabel(item.SizeBytes) }},
	{Header: "USED", Value: func(item runtime.VolumeEntry) string { return volumeSizeLabel(item.UsedBytes) }},
	{Header: "SOURCE", Value: func(item runtime.VolumeEntry) string { return volumeSourceLabel(item.Source) }},
}

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Manage Nedgentic storage",
}

var storageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Nedgentic storage volumes",
	Run: func(cmd *cobra.Command, _ []string) {
		flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
		logging.SetLevel(logging.ParseLevel(flags.LogLevel))
		bin, err := runtime.ResolveContainerBin(flags)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		entries, err := runtime.ListVolumes(bin)
		if err != nil {
			output.Printf("Error listing storage: %v", err)
			os.Exit(1)
		}
		filtered := filterStorageEntries(entries, storageAll)
		summary := collectStorageSummary(bin)
		response := listResponse[runtime.VolumeEntry]{Kind: "storage_volume_list", Items: filtered, Summary: summary}
		if err := renderOutput(cmd, response, func(writer io.Writer) error {
			if err := writeItemTable(writer, filtered, storageTableFields, "No Nedgentic storage volumes found."); err != nil {
				return err
			}
			return writeStorageSummaryText(writer, summary)
		}); err != nil {
			output.Printf("Error rendering storage: %v", err)
			os.Exit(1)
		}
	},
}

var storagePruneAllImages bool

var storagePruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune stopped VMs, build caches, and dangling images",
	Run: func(cmd *cobra.Command, _ []string) {
		flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
		logging.SetLevel(logging.ParseLevel(flags.LogLevel))
		bin, err := runtime.ResolveContainerBin(flags)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		if err := runWithSpinner("Prune stopped VMs:", func() error {
			return runtime.ContainerPrune(bin)
		}); err != nil {
			output.Printf("Error pruning stopped VMs: %v", err)
			os.Exit(1)
		}
		if err := runWithSpinner("Prune images:", func() error {
			return runtime.ImagePrune(bin, storagePruneAllImages)
		}); err != nil {
			output.Printf("Error pruning images: %v", err)
			os.Exit(1)
		}
		if err := runWithSpinner("Remove build cache:", func() error {
			return runtime.BuilderDelete(bin)
		}); err != nil {
			output.Printf("Error removing build cache: %v", err)
			os.Exit(1)
		}
		output.Print("Storage prune complete: ok")
	},
}

func init() {
	storageListCmd.Flags().BoolVar(&storageAll, "all", false, "Include all volumes, not just Ned-managed")
	addOutputFlag(storageListCmd)
	storageCmd.AddCommand(storageListCmd)
	storagePruneCmd.Flags().BoolVar(&storagePruneAllImages, "all-images", false, "Remove all unused images, not just dangling ones")
	storageCmd.AddCommand(storagePruneCmd)
	rootCmd.AddCommand(storageCmd)
}

type storageRow struct {
	Name   string
	Size   string
	Used   string
	Source string
}

func buildStorageRows(entries []runtime.VolumeEntry, includeAll bool) []storageRow {
	rows := make([]storageRow, 0, len(entries))
	for _, entry := range entries {
		if !includeAll && !strings.HasPrefix(entry.Name, nedDiskVolumePrefix) {
			continue
		}
		size := "-"
		if entry.SizeBytes > 0 {
			size = runtime.FormatBytes(entry.SizeBytes)
		}
		used := "-"
		if entry.UsedBytes > 0 {
			used = runtime.FormatBytes(entry.UsedBytes)
		}
		source := "-"
		if strings.TrimSpace(entry.Source) != "" {
			source = entry.Source
		}
		rows = append(rows, storageRow{
			Name:   entry.Name,
			Size:   size,
			Used:   used,
			Source: source,
		})
	}
	return rows
}

func filterStorageEntries(entries []runtime.VolumeEntry, includeAll bool) []runtime.VolumeEntry {
	filtered := make([]runtime.VolumeEntry, 0, len(entries))
	for _, entry := range entries {
		if !includeAll && !strings.HasPrefix(entry.Name, nedDiskVolumePrefix) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func volumeSizeLabel(value int64) string {
	if value <= 0 {
		return "-"
	}
	return runtime.FormatBytes(value)
}

func volumeSourceLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
