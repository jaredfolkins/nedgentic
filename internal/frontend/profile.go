package frontend

import (
	"io"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage profiles",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		home, err := profile.ResolveHome()
		if err != nil {
			output.Printf("Error resolving home: %v", err)
			os.Exit(1)
		}
		if err := profile.EnsureProfileLayout(home); err != nil {
			output.Printf("Error ensuring profile layout: %v", err)
			os.Exit(1)
		}
		names, err := profileNames(profile.PersonalProfilesDir(home))
		if err != nil {
			output.Printf("Error listing personal profiles: %v", err)
			os.Exit(1)
		}
		items := make([]profileListItem, 0, len(names))
		for _, name := range names {
			items = append(items, profileListItem{Name: name})
		}
		response := listResponse[profileListItem]{Kind: "profile_list", Items: items}
		if err := renderOutput(cmd, response, func(writer io.Writer) error {
			if _, err := io.WriteString(writer, "Personal Profiles:\n"); err != nil {
				return err
			}
			return writeProfileNames(writer, names)
		}); err != nil {
			output.Printf("Error rendering profiles: %v", err)
			os.Exit(1)
		}
	},
}

type profileListItem struct {
	Name string `json:"name"`
}

func profileNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func writeProfileNames(writer io.Writer, names []string) error {
	if len(names) == 0 {
		_, err := io.WriteString(writer, "  (none)\n")
		return err
	}
	for _, name := range names {
		if _, err := io.WriteString(writer, "  - "+name+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	addOutputFlag(profileListCmd)
	profileCmd.AddCommand(profileListCmd)
	rootCmd.AddCommand(profileCmd)
}
