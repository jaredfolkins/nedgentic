package frontend

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
)

var stagingCmd = &cobra.Command{
	Use:   "staging",
	Short: "Manage staging mounts",
}

var stagingListTableFields = []tableField[stagingListItem]{
	{Header: "VM", Value: func(item stagingListItem) string { return item.VM }},
	{Header: "PATH", Value: func(item stagingListItem) string { return item.Path }},
}

var stagingListCmd = &cobra.Command{
	Use:   "list",
	Short: "List staging directories",
	Run: func(cmd *cobra.Command, args []string) {
		names, base, err := runtime.ListStagingDirs()
		if err != nil {
			output.Printf("Error listing staging directories: %v", err)
			os.Exit(1)
		}
		items := make([]stagingListItem, 0, len(names))
		for _, name := range names {
			items = append(items, stagingListItem{
				VM:   name,
				Path: filepath.Join(base, name),
			})
		}
		response := stagingListResponse{Kind: "staging_dir_list", Base: base, Items: items}
		if err := renderOutput(cmd, response, func(writer io.Writer) error {
			return writeItemTable(writer, items, stagingListTableFields, "No staging directories found.")
		}); err != nil {
			output.Printf("Error rendering staging directories: %v", err)
			os.Exit(1)
		}
	},
}

type stagingListItem struct {
	VM   string `json:"vm"`
	Path string `json:"path"`
}

type stagingListResponse struct {
	Kind  string            `json:"kind"`
	Base  string            `json:"base"`
	Items []stagingListItem `json:"items"`
}

var stagingCleanAll bool

var stagingCleanCmd = &cobra.Command{
	Use:   "clean [vm]...",
	Short: "Remove staging directories",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if stagingCleanAll {
			if err := runtime.CleanStagingAll(); err != nil {
				output.Printf("Error cleaning staging directories: %v", err)
				os.Exit(1)
			}
			output.Print("Staging directories removed.")
			return
		}
		if len(args) == 0 {
			output.Print("VM name is required (or use --all)")
			os.Exit(1)
		}
		var failed bool
		for _, raw := range args {
			name := strings.TrimSpace(raw)
			if name == "" {
				output.Print("VM name is required")
				failed = true
				continue
			}
			if err := runtime.RemoveStagingDir(name); err != nil {
				output.Printf("Error removing staging for %s: %v", name, err)
				failed = true
			}
		}
		if failed {
			os.Exit(1)
		}
	},
}

func init() {
	stagingCleanCmd.Flags().BoolVar(&stagingCleanAll, "all", false, "Remove all staging directories")
	addOutputFlag(stagingListCmd)
	stagingCmd.AddCommand(stagingListCmd)
	stagingCmd.AddCommand(stagingCleanCmd)
	rootCmd.AddCommand(stagingCmd)
}
