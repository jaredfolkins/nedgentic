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
	Use:   "gentic",
	Short: "Start a sandboxed VM and attach a shell",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var genticPersonalCmd = &cobra.Command{
	Use:     "personal <profile> [name]",
	Aliases: []string{"p"},
	Short:   "Start a personal profile VM and attach a shell",
	Args:    cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runProfileFromArgs(cmd, args); err != nil {
			output.Printf("Error: %v", err)
			os.Exit(1)
		}
	},
}

var flagHostBinaries []string
var flagHookDir string
var flagHerdr bool

func init() {
	genticCmd.PersistentFlags().StringArrayVarP(&flagHostBinaries, "host-bin", "b", nil, "Copy a host binary into /usr/local/bin (repeatable)")
	genticCmd.PersistentFlags().StringVar(&flagHookDir, "hook-dir", "", "Mount and use a host hook directory for this VM launch")
	genticCmd.PersistentFlags().BoolVar(&flagHerdr, "herdr", false, "Run the attached shell in a persistent HerdR workspace")
	genticCmd.AddCommand(genticPersonalCmd)
	rootCmd.AddCommand(genticCmd)
}

func runProfileFromArgs(cmd *cobra.Command, args []string) error {
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
	return runProfile(profileName, containerNameOverride, cmd)
}

func runProfile(profileName, vmNameOverride string, cmd *cobra.Command) error {
	return runProfileWithImage(profileName, vmNameOverride, "", cmd)
}

func runProfileWithImage(profileName, vmNameOverride, imageOverride string, cmd *cobra.Command) error {
	profilePath := strings.TrimSpace(flagProfilePath)
	if profilePath == "" {
		home, err := profile.ResolveHome()
		if err != nil {
			return err
		}
		profilePath = profile.ProfileConfigPath(home, profileName)
	}
	prof, err := runtime.LoadProfile(profilePath, true, profileName)
	if err != nil {
		return err
	}
	if err := runtime.ApplyHostBinaries(prof, flagHostBinaries); err != nil {
		return err
	}
	flags := runtime.EffectiveFlags(prof)
	flags = applyFlagOverrides(flags, cmd)
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
	if strings.TrimSpace(imageOverride) != "" {
		image = strings.TrimSpace(imageOverride)
	}
	logging.Debugf("Profile %s uses image: %s", profileName, image)
	profileDir := filepath.Dir(profilePath)

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
	hookDirOverride := strings.TrimSpace(flagHookDir)
	exists, status, err := runtime.ContainerStatusByName(bin, containerName)
	if err != nil {
		return err
	}
	if exists && status == "running" {
		if hookDirOverride != "" {
			logging.Warnf("hook-dir is ignored when VM %s is already running; restart the VM to apply hook changes", containerName)
		}
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
	hookSourceDir, forceHookMount, err := resolveGenticHookSource(profileDir, hookDirOverride, hostUser.Home)
	if err != nil {
		return err
	}
	if forceHookMount {
		flags.GenticDir = runtime.GenticMountDir
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
		if hookSourceDir != "" {
			if err := runtime.AppendMount(plan, runtime.ContainerMount{
				Source:   hookSourceDir,
				Target:   runtime.GenticMountDir,
				ReadOnly: true,
			}); err != nil {
				if cleanup != nil {
					cleanup()
				}
				return err
			}
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
				// An auto-removed VM is ephemeral, so its logins go with it.
				return runtime.DeleteAgentAuthVolume(opts.Bin, opts.ContainerName)
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

func resolveGenticHookSource(profileDir, hookDir, hostHome string) (string, bool, error) {
	if strings.TrimSpace(hookDir) != "" {
		resolved, err := resolveHookDir(hookDir, hostHome)
		if err != nil {
			return "", false, err
		}
		return resolved, true, nil
	}
	hasGenticScripts, err := runtime.GenticScriptsPresent(profileDir)
	if err != nil {
		return "", false, err
	}
	if !hasGenticScripts {
		return "", false, nil
	}
	return filepath.Join(profileDir, profile.GenticDirName), false, nil
}

func resolveHookDir(raw, hostHome string) (string, error) {
	value, err := normalizeAgentPath("hook dir", raw)
	if err != nil {
		return "", err
	}
	resolved, err := resolveHookPath(value)
	if err != nil {
		return "", fmt.Errorf("hook dir resolution failed: %w", err)
	}
	if err := ensureHookPathUnderHome(resolved, hostHome); err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("hook dir must be a directory: %s", resolved)
	}
	hasScripts, err := hasHookScripts(resolved)
	if err != nil {
		return "", err
	}
	if !hasScripts {
		return "", fmt.Errorf("hook dir must contain at least one .sh hook: %s", resolved)
	}
	return resolved, nil
}

func resolveHookPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func ensureHookPathUnderHome(path, home string) error {
	homeResolved, err := resolveHookPath(home)
	if err != nil {
		return fmt.Errorf("host home is required: %w", err)
	}
	rel, err := filepath.Rel(homeResolved, path)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("hook dir must be under host home: %s", path)
	}
	return nil
}

func hasHookScripts(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sh") {
			return true, nil
		}
	}
	return false, nil
}
