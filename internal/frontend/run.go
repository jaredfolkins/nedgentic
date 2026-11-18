package frontend

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var genticCmd = &cobra.Command{
	Use:   "gentic <profile> [vm]",
	Short: "Start a profile's VM and attach a shell",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runProfileFromArgs(args); err != nil {
			output.Printf("Error: %v", err)
			os.Exit(1)
		}
	},
}

var flagHerdr bool

func init() {
	genticCmd.Flags().BoolVar(&flagHerdr, "herdr", false, "Run the attached shell in a persistent HerdR workspace")
	rootCmd.AddCommand(genticCmd)
}

func runProfileFromArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("profile name is required")
	}
	profileName, err := normalizeAgentName("profile name", args[0])
	if err != nil {
		return err
	}
	containerNameOverride := ""
	if len(args) > 1 {
		containerNameOverride, err = normalizeAgentName("vm name", args[1])
		if err != nil {
			return err
		}
	}
	if handled, err := maybeRunProfileInHerdr(profileName, containerNameOverride); handled || err != nil {
		return err
	}
	return runProfile(profileName, containerNameOverride)
}

func runProfile(profileName, vmNameOverride string) error {
	home, err := profile.ResolveHome()
	if err != nil {
		return err
	}
	profilePath := profile.ProfileConfigPath(home, profileName)
	prof, err := runtime.LoadProfile(profilePath, true, profileName)
	if err != nil {
		return err
	}
	if err := runtime.ApplyHostBinaries(prof); err != nil {
		return err
	}
	flags := runtime.EffectiveFlags(prof)
	logging.SetLevel(logging.ParseLevel(flags.LogLevel))
	stepper := runtime.NewStepTimer()
	if err := validateLaunchFlags(flags); err != nil {
		return err
	}

	containerName, err := runtime.ContainerNameForProfile(profileName)
	if err != nil {
		return err
	}
	if vmNameOverride != "" {
		containerName = vmNameOverride
	}
	bin, err := runtime.ResolveContainerBin(flags)
	if err != nil {
		return err
	}
	image, err := runtime.ResolveRunImage(prof)
	if err != nil {
		return err
	}
	logging.Debugf("Profile %s uses image: %s", profileName, image)

	hostUser, err := runtime.ResolveHostUser()
	if err != nil {
		return err
	}
	shellPath, err := runtime.ResolveShellPath(prof)
	if err != nil {
		return err
	}
	hostWorkdir, err := os.Getwd()
	if err != nil {
		return err
	}
	workdir := hostWorkdir
	if mapped, ok, err := runtime.MapHostPathToGuest(hostWorkdir, hostUser.Home, hostUser.Name); err != nil {
		return err
	} else if ok {
		workdir = filepath.Clean(mapped)
	}
	exists, status, err := runtime.ContainerStatusByName(bin, containerName)
	if err != nil {
		return err
	}
	if exists && status == "running" {
		return stepper.Run("Attach to running VM", func() error {
			output.Printf("VM %s already running, attaching...", containerName)
			attachWorkdir, err := runtime.ResolveRunningWorkdir(bin, containerName, hostUser)
			if err != nil {
				return err
			}
			err = attachWithSessionLock(AttachOptions{
				Bin:           bin,
				ContainerName: containerName,
				ProfileName:   profileName,
				Flags:         flags,
				ShellPath:     shellPath,
				Workdir:       attachWorkdir,
				HostUser:      hostUser,
			})
			if err != nil {
				logging.Warnf("Failed to attach in %s: %v. Falling back to home directory.", attachWorkdir, err)
				fallbackDir := runtime.GuestHomeForUser(hostUser.Name)
				if fallbackDir == attachWorkdir {
					return err
				}
				return attachWithSessionLock(AttachOptions{
					Bin:           bin,
					ContainerName: containerName,
					ProfileName:   profileName,
					Flags:         flags,
					ShellPath:     shellPath,
					Workdir:       fallbackDir,
					HostUser:      hostUser,
				})
			}
			return nil
		})
	}
	if exists {
		if err := stepper.Run("Remove stale VM", func() error {
			return runtime.ContainerDelete(bin, containerName)
		}); err != nil {
			return err
		}
	}

	if err := stepper.Run("Ensure OCI image", func() error {
		return ensureImagePull(bin, image, flags.Platform)
	}); err != nil {
		return err
	}

	var plan *runtime.MountPlan
	var cleanup func()
	var stagingRoot string
	var runnerDir string
	if err := stepper.Run("Stage mounts", func() error {
		var err error
		stagingRoot, err = runtime.EnsureStagingDir(containerName, true)
		if err != nil {
			return err
		}
		plan, cleanup, err = runtime.PlanMounts(prof.Files, prof.Directories, hostUser.Name, hostUser.Home, stagingRoot)
		if err != nil {
			return err
		}
		runnerDir, err = runtime.EnsureRunnerDir(stagingRoot)
		if err != nil {
			if cleanup != nil {
				cleanup()
			}
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cleanup()
		os.Exit(1)
	}()
	defer signal.Stop(sigCh)

	hostname := containerName
	output.Printf("Booting: host=%s profile=%s [%.3fs]", hostname, profilePath, stepper.Elapsed().Seconds())
	containerWorkdir := ""
	if plan != nil {
		containerWorkdir = plan.WorkspaceMount
	}
	if containerWorkdir == "" {
		containerWorkdir = workdir
	}
	if err := stepper.Run("Start persistent VM", func() error {
		return runtime.RunContainer(runtime.RunContainerOptions{
			Bin:              bin,
			Image:            image,
			ContainerName:    containerName,
			ProfileName:      profileName,
			Hostname:         hostname,
			Flags:            flags,
			Plan:             plan,
			HostUser:         hostUser,
			ShellPath:        shellPath,
			DesiredWorkdir:   workdir,
			ContainerWorkdir: containerWorkdir,
			RunnerDir:        runnerDir,
			Detached:         true,
			KeepAlive:        true,
		})
	}); err != nil {
		return err
	}

	startupLogsPrinted := false
	printStartupLogs := func() {
		if startupLogsPrinted {
			return
		}
		if printContainerStartupLogs(os.Stdout, runtime.ContainerLogs, bin, containerName) {
			startupLogsPrinted = true
		}
	}

	if flags.Systemd != nil && *flags.Systemd {
		if err := stepper.Run("Wait for systemd", func() error {
			err := runWithSpinnerStep("Wait for systemd", func() error {
				return runtime.WaitForContainerSystemdReady(bin, containerName)
			})
			if err != nil {
				printStartupLogs()
			}
			return err
		}); err != nil {
			return err
		}
		if err := stepper.Run("Bootstrap systemd VM", func() error {
			err := runtime.BootstrapContainerSystemd(bin, containerName)
			printStartupLogs()
			return err
		}); err != nil {
			return err
		}
	} else {
		if err := stepper.Run("Wait for ready", func() error {
			err := runWithSpinnerStep("Wait for ready", func() error {
				return runtime.WaitForContainerReady(bin, containerName)
			})
			printStartupLogs()
			return err
		}); err != nil {
			return err
		}
	}

	return attachWithSessionLock(AttachOptions{
		Bin:           bin,
		ContainerName: containerName,
		ProfileName:   profileName,
		Flags:         flags,
		ShellPath:     shellPath,
		Workdir:       workdir,
		HostUser:      hostUser,
	})
}

type containerLogFetcher func(bin, containerName string) (string, error)

func printContainerStartupLogs(w io.Writer, fetch containerLogFetcher, bin, containerName string) bool {
	if w == nil || fetch == nil {
		return false
	}
	logs, err := fetch(bin, containerName)
	if err != nil {
		return false
	}
	logs = strings.TrimSpace(logs)
	if logs == "" {
		return false
	}
	_, _ = fmt.Fprintln(w, logs)
	return true
}

func validateLaunchFlags(flags profile.Flags) error {
	if flags.Systemd == nil || !*flags.Systemd {
		return nil
	}
	if flags.Init != nil && *flags.Init {
		return fmt.Errorf("--systemd cannot be combined with --init")
	}
	if strings.TrimSpace(flags.InitImage) != "" {
		return fmt.Errorf("--systemd cannot be combined with --init-image")
	}
	return nil
}

type AttachOptions struct {
	Bin           string
	ContainerName string
	ProfileName   string
	Flags         profile.Flags
	ShellPath     string
	Workdir       string
	HostUser      *runtime.HostUser
}

func attachWithSessionLock(opts AttachOptions) error {
	var lock *runtime.SessionLock
	if opts.Flags.RemoveContainerOnExit != nil && *opts.Flags.RemoveContainerOnExit {
		var err error
		lock, err = runtime.AcquireSessionLock(opts.ContainerName)
		if err != nil {
			return err
		}
	}
	output.Printf("Attached: profile=%s vm=%s", opts.ProfileName, opts.ContainerName)
	err := runtime.ExecContainerShell(runtime.ExecShellOptions{
		Bin:           opts.Bin,
		ContainerName: opts.ContainerName,
		ShellPath:     opts.ShellPath,
		Workdir:       opts.Workdir,
		Flags:         opts.Flags,
		HostUser:      opts.HostUser,
		Ulimits:       opts.Flags.Ulimits,
	})
	if err != nil {
		output.Printf("Detached: profile=%s vm=%s (error)", opts.ProfileName, opts.ContainerName)
	} else {
		output.Printf("Detached: profile=%s vm=%s", opts.ProfileName, opts.ContainerName)
	}
	if lock != nil {
		exclusive, lockErr := lock.TryUpgradeExclusive()
		if lockErr != nil {
			logging.Warnf("Failed to update session lock for %s: %v", opts.ContainerName, lockErr)
		} else if exclusive {
			label := fmt.Sprintf("Auto-remove: profile=%s vm=%s (last session)", opts.ProfileName, opts.ContainerName)
			if removeErr := runWithSpinner(label, func() error {
				if err := runtime.RemoveContainerIfIdle(opts.Bin, opts.ContainerName); err != nil {
					return err
				}
				// An auto-removed VM is ephemeral, so its logins and staging go with it.
				if err := runtime.DeleteAgentAuthVolume(opts.Bin, opts.ContainerName); err != nil {
					return err
				}
				return runtime.RemoveStagingDir(opts.ContainerName)
			}); removeErr != nil {
				logging.Warnf("Failed to remove VM %s after session exit: %v", opts.ContainerName, removeErr)
			}
		}
		if releaseErr := lock.Release(); releaseErr != nil {
			logging.Warnf("Failed to release session lock for %s: %v", opts.ContainerName, releaseErr)
		}
	}
	return err
}
