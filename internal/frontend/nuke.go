package frontend

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var nukeYes bool

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Stop and delete Nedgentic resources (VMs, images)",
}

var nukeVmsCmd = &cobra.Command{
	Use:     "vms",
	Aliases: []string{"vm"},
	Short:   "Stop and delete all Nedgentic-managed VMs",
	Run: func(cmd *cobra.Command, args []string) {
		runNuke(cmd, true, false)
	},
}

var nukeImagesCmd = &cobra.Command{
	Use:     "images",
	Aliases: []string{"image", "img"},
	Short:   "Delete all images",
	Run: func(cmd *cobra.Command, args []string) {
		runNuke(cmd, false, true)
	},
}

var nukeAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Stop and delete all Nedgentic-managed VMs AND images",
	Run: func(cmd *cobra.Command, args []string) {
		runNuke(cmd, true, true)
	},
}

func init() {
	nukeCmd.PersistentFlags().BoolVarP(&nukeYes, "yes", "y", false, "Skip confirmation prompt")
	nukeCmd.AddCommand(nukeVmsCmd)
	nukeCmd.AddCommand(nukeImagesCmd)
	nukeCmd.AddCommand(nukeAllCmd)
	rootCmd.AddCommand(nukeCmd)
}

func runNuke(cmd *cobra.Command, doVMs, doImages bool) {
	flags := profile.DefaultFlags()
	flags = applyFlagOverrides(flags, cmd)
	logging.SetLevel(logging.ParseLevel(flags.LogLevel))

	bin, err := runtime.ResolveContainerBin(flags)
	if err != nil {
		printRuntimeResolveError(err)
		os.Exit(1)
	}

	var managed []runtime.ContainerListEntry
	if doVMs {
		containers, err := runtime.ListContainers(bin)
		if err != nil {
			output.Printf("Error listing VMs: %v", err)
			os.Exit(1)
		}

		for _, c := range containers {
			if c.Configuration.Labels[runtime.ContainerLabelKey] == "1" {
				managed = append(managed, c)
			}
		}
	}

	var images []runtime.ImageListEntry
	if doImages {
		allImages, err := runtime.ListImages(bin)
		if err != nil {
			output.Printf("Error listing images: %v", err)
			os.Exit(1)
		}
		images = allImages
	}

	if len(managed) == 0 && len(images) == 0 {
		if doVMs && doImages {
			output.Print("No Nedgentic-managed VMs or images found.")
		} else if doVMs {
			output.Print("No Nedgentic-managed VMs found.")
		} else {
			output.Print("No images found.")
		}
		return
	}

	if len(managed) > 0 {
		output.Print("Found Nedgentic-managed VMs:")
		rows := buildListRows(managed, true)
		var buf strings.Builder
		w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tPROFILE\tUSER\tSTATUS\tHOSTNAME\tIP\tWORKSPACE\tIMAGE")
		for _, row := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				row.Name,
				row.Profile,
				row.User,
				row.Status,
				row.Hostname,
				row.IP,
				row.Workspace,
				row.Image,
			)
		}
		w.Flush()
		for line := range strings.SplitSeq(strings.TrimRight(buf.String(), "\n"), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			output.Print(line)
		}
	}

	if len(images) > 0 {
		output.Print("Found Images:")
		var buf strings.Builder
		w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "REPOSITORY\tTAG\tIMAGE ID\tSIZE")
		for _, img := range images {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", img.Repository, img.Tag, img.ID, img.Size)
		}
		w.Flush()
		for line := range strings.SplitSeq(strings.TrimRight(buf.String(), "\n"), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			output.Print(line)
		}
	}

	if !nukeYes {
		target := "items"
		if doVMs && doImages {
			target = "VMs and Images"
		} else if doVMs {
			target = "VMs"
		} else if doImages {
			target = "Images"
		}
		confirmed, err := confirmMutation(fmt.Sprintf("Are you sure you want to STOP and DELETE all these %s?", target), nukeYes)
		if err != nil {
			output.Printf("Nuke failed: %v", err)
			os.Exit(1)
		}
		if !confirmed {
			output.Print("Aborted.")
			return
		}
	}

	if len(managed) > 0 {
		for _, c := range managed {
			stopErr := runWithSpinner(fmt.Sprintf("Stop VM: %s", c.Configuration.ID), func() error {
				var err error
				for i := range 3 {
					err = runtime.ContainerStop(bin, c.Configuration.ID)
					if err == nil {
						return nil
					}
					if i < 2 {
						time.Sleep(1 * time.Second)
					}
				}
				return err
			})
			if stopErr != nil {
				output.Printf("Warning: stop failed for %s after 3 attempts: %v", c.Configuration.ID, stopErr)
			}

			if err := runWithSpinner(fmt.Sprintf("Delete VM: %s", c.Configuration.ID), func() error {
				return runtime.ContainerDelete(bin, c.Configuration.ID)
			}); err != nil {
				output.Printf("Error deleting %s: %v", c.Configuration.ID, err)
			}
			if err := runWithSpinner(fmt.Sprintf("Remove staging: %s", c.Configuration.ID), func() error {
				return runtime.RemoveStagingDir(c.Configuration.ID)
			}); err != nil {
				output.Printf("Warning: failed to remove staging for %s: %v", c.Configuration.ID, err)
			}
		}
		if err := runWithSpinner("Clean staging base: ~/.ned/staging", runtime.CleanStagingAll); err != nil {
			output.Printf("Warning: failed to remove staging base: %v", err)
		}
	}

	if len(images) > 0 {
		for _, img := range images {
			target := img.ID
			if img.Repository != "" && img.Tag != "" && img.Tag != "<none>" {
				target = fmt.Sprintf("%s:%s", img.Repository, img.Tag)
			}
			if err := runWithSpinner(fmt.Sprintf("Delete image: %s", target), func() error {
				return runtime.ContainerImageDelete(bin, target)
			}); err != nil {
				output.Printf("Warning: failed to delete image %s: %v", target, err)
			}
		}
	}

	output.Print("Nuke complete: ok")
}
