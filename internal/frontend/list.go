package frontend

import (
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var listAll = true

var listTableFields = []tableField[listRow]{
	{Header: "NAME", Value: func(item listRow) string { return item.Name }},
	{Header: "PROFILE", Value: func(item listRow) string { return item.Profile }},
	{Header: "USER", Value: func(item listRow) string { return item.User }},
	{Header: "STATUS", Value: func(item listRow) string { return item.Status }},
	{Header: "HOSTNAME", Value: func(item listRow) string { return item.Hostname }},
	{Header: "IP", Value: func(item listRow) string { return item.IP }},
	{Header: "WORKSPACE", Value: func(item listRow) string { return item.Workspace }},
	{Header: "IMAGE", Value: func(item listRow) string { return item.Image }},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List Nedgentic VMs",
	Run: func(cmd *cobra.Command, args []string) {
		flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
		logging.SetLevel(logging.ParseLevel(flags.LogLevel))
		bin, err := runtime.ResolveContainerBin(flags)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		entries, err := runtime.ListContainers(bin)
		if err != nil {
			output.Printf("Error listing VMs: %v", err)
			os.Exit(1)
		}
		rows := buildListRows(entries, listAll)
		summary := collectStorageSummary(bin)
		response := listResponse[listRow]{Kind: "vm_list", Items: rows, Summary: summary}
		if err := renderOutput(cmd, response, func(writer io.Writer) error {
			if err := writeItemTable(writer, rows, listTableFields, "No Nedgentic VMs found."); err != nil {
				return err
			}
			return writeStorageSummaryText(writer, summary)
		}); err != nil {
			output.Printf("Error rendering VM list: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	listCmd.Flags().BoolVar(&listAll, "all", true, "Include stopped VMs")
	addOutputFlag(listCmd)
	rootCmd.AddCommand(listCmd)
}

type listRow struct {
	Name      string `json:"name"`
	Profile   string `json:"profile"`
	Image     string `json:"image"`
	Status    string `json:"status"`
	Hostname  string `json:"hostname"`
	IP        string `json:"ip"`
	Workspace string `json:"workspace"`
	User      string `json:"user"`
}

func buildListRows(entries []runtime.ContainerListEntry, includeStopped bool) []listRow {
	rows := make([]listRow, 0, len(entries))
	for _, entry := range entries {
		labels := entry.Configuration.Labels
		if labels[runtime.ContainerLabelKey] != "1" {
			continue
		}
		if !includeStopped && !isRunningStatus(entry.Status) {
			continue
		}

		ip := containerIPv4(entry)
		image := labels[runtime.LabelImageKey]
		if strings.TrimSpace(image) == "" {
			image = entry.Configuration.Image.Reference
		}

		rows = append(rows, listRow{
			Name:      fieldOrDashIdentifier("vm name", entry.Configuration.ID),
			Profile:   fieldOrDashIdentifier("profile name", labels[runtime.LabelProfileKey]),
			Image:     fieldOrDashReference("image reference", image),
			Status:    fieldOrDashText(entry.Status),
			Hostname:  fieldOrDashIdentifier("hostname", labels[runtime.LabelHostnameKey]),
			IP:        fieldOrDashText(ip),
			Workspace: fieldOrDashPath("workspace path", labels[runtime.LabelWorkspaceKey]),
			User:      fieldOrDashIdentifier("user", labels[runtime.LabelUserKey]),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func isRunningStatus(status string) bool {
	value := strings.ToLower(strings.TrimSpace(status))
	return strings.HasPrefix(value, "running") || strings.HasPrefix(value, "up")
}
