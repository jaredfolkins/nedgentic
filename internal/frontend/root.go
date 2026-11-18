package frontend

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var (
	flagLogLevel            string
	flagContainerBin        string
	flagOutput              string
	flagInit                bool
	flagInitImage           string
	flagSystemd             bool
	flagNetwork             string
	flagProxy               string
	flagHTTPProxy           string
	flagHTTPSProxy          string
	flagNoProxy             []string
	flagNoDNS               bool
	flagDNS                 []string
	flagDNSDomain           string
	flagDNSSearch           []string
	flagDNSOptions          []string
	flagRemoveContainerExit bool
	flagProfilePath         string
	flagGenticDir           string
	flagGenticDebug         bool
	flagCPUs                string
	flagMemory              string
	flagDisk                string
	flagPlatform            string
	flagUlimits             []string
	flagCapAdd              []string
	flagCapDrop             []string
	flagBrevity             bool
)

var rootCmd = &cobra.Command{
	Use:   "ned",
	Short: "Ned: CLI for Nedgentic",
	Long:  "Ned runs OCI images in lightweight VMs (via the container runtime) and drops you into a sandboxed shell with your profiles mounted. Supports multi-arch VMs via --platform. Use `ned gentic personal <profile>` to start.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := configureOutput(); err != nil {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			return err
		}
		if shouldSkipDependencyCheck(cmd) {
			return nil
		}
		// Apply flags to get bin
		flags := profile.DefaultFlags()
		flags = applyFlagOverrides(flags, cmd)
		if err := ensureDependencies(cmd, flags); err != nil {
			output.Printf("Dependency check failed: %v", err)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			return err
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func Execute(buildTime string) {
	if err := configureOutput(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Invalid output: %v\n", err)
		os.Exit(1)
	}
	if buildTime != "dev" {
		output.Printf("Build date: %s", buildTime)
	}
	logging.SetLevel(logging.ParseLevel(profile.DefaultFlags().LogLevel))
	os.Args = normalizeArgs(os.Args)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringVarP(&flagContainerBin, "container-bin", "c", "", "Container runtime binary")
	rootCmd.PersistentFlags().BoolVar(&flagInit, "init", false, "Run an init process in container-backed VMs")
	rootCmd.PersistentFlags().StringVar(&flagInitImage, "init-image", "", "Custom init image for container-backed VMs")
	rootCmd.PersistentFlags().BoolVar(&flagSystemd, "systemd", false, "Run /sbin/init as PID 1 in container-backed VMs")
	rootCmd.PersistentFlags().StringVar(&flagNetwork, "network", "", "Attach VM to a named container network")
	rootCmd.PersistentFlags().StringVar(&flagProxy, "proxy", "", "Set both HTTP and HTTPS proxy URL for VM commands")
	rootCmd.PersistentFlags().StringVar(&flagHTTPProxy, "http-proxy", "", "Set HTTP proxy URL for VM commands")
	rootCmd.PersistentFlags().StringVar(&flagHTTPSProxy, "https-proxy", "", "Set HTTPS proxy URL for VM commands")
	rootCmd.PersistentFlags().StringArrayVar(&flagNoProxy, "no-proxy", nil, "Set proxy bypass host/domain/CIDR (repeatable)")
	rootCmd.PersistentFlags().BoolVar(&flagNoDNS, "no-dns", false, "Disable runtime-managed DNS")
	rootCmd.PersistentFlags().StringArrayVar(&flagDNS, "dns", nil, "Set DNS server (repeatable)")
	rootCmd.PersistentFlags().StringVar(&flagDNSDomain, "dns-domain", "", "Set DNS domain")
	rootCmd.PersistentFlags().StringArrayVar(&flagDNSSearch, "dns-search", nil, "Set DNS search domain (repeatable)")
	rootCmd.PersistentFlags().StringArrayVar(&flagDNSOptions, "dns-option", nil, "Set DNS options (repeatable)")
	rootCmd.PersistentFlags().StringVarP(&flagLogLevel, "log-level", "l", "", "Log level (debug|info|warn|error)")
	rootCmd.PersistentFlags().StringVarP(&flagProfilePath, "profile-path", "p", "", "Explicit profile config path")
	rootCmd.PersistentFlags().BoolVar(&flagRemoveContainerExit, "rm-container-on-exit", false, "Remove VM after exit")
	rootCmd.PersistentFlags().StringVar(&flagGenticDir, "gentic-dir", "", "Override the guest gentic hooks directory")
	rootCmd.PersistentFlags().BoolVar(&flagGenticDebug, "gentic-debug", false, "Enable gentic hook debug tracing")
	rootCmd.PersistentFlags().StringVar(&flagCPUs, "cpus", "", "CPUs to allocate to the VM (e.g. 2 or 1.5)")
	rootCmd.PersistentFlags().StringVar(&flagMemory, "memory", "", "Memory to allocate to the VM (e.g. 4G, 512M)")
	rootCmd.PersistentFlags().StringVar(&flagDisk, "disk", "", "Disk size for the VM data volume (e.g. 40G)")
	rootCmd.PersistentFlags().StringVar(&flagPlatform, "platform", "", "Platform for the image/VM (e.g. linux/amd64, linux/arm64)")
	rootCmd.PersistentFlags().StringArrayVar(&flagUlimits, "ulimit", nil, "Set process resource limits (repeatable: <type>=<soft>[:<hard>])")
	rootCmd.PersistentFlags().StringArrayVar(&flagCapAdd, "cap-add", nil, "Add a Linux capability to container-backed VMs (repeatable)")
	rootCmd.PersistentFlags().StringArrayVar(&flagCapDrop, "cap-drop", nil, "Drop a Linux capability from container-backed VMs (repeatable)")
	rootCmd.PersistentFlags().BoolVar(&flagBrevity, "brevity", false, "Show concise help output instead of the full command tree")
}

func normalizeArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	out := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case arg == "-rm":
			out = append(out, "--rm-container-on-exit")
		case strings.HasPrefix(arg, "-rm="):
			out = append(out, "--rm-container-on-exit="+strings.TrimPrefix(arg, "-rm="))
		default:
			out = append(out, arg)
		}
	}
	return out
}

func applyFlagOverrides(flags profile.Flags, cmd *cobra.Command) profile.Flags {
	if cmd == nil {
		return flags
	}
	if cmd.Flags().Changed("log-level") {
		flags.LogLevel = strings.TrimSpace(flagLogLevel)
	}
	if cmd.Flags().Changed("container-bin") {
		flags.ContainerBin = strings.TrimSpace(flagContainerBin)
	}
	if cmd.Flags().Changed("init") {
		value := flagInit
		flags.Init = &value
	}
	if cmd.Flags().Changed("init-image") {
		flags.InitImage = strings.TrimSpace(flagInitImage)
	}
	if cmd.Flags().Changed("systemd") {
		value := flagSystemd
		flags.Systemd = &value
	}
	if cmd.Flags().Changed("network") {
		flags.Network = strings.TrimSpace(flagNetwork)
	}
	if cmd.Flags().Changed("proxy") {
		flags.Proxy = strings.TrimSpace(flagProxy)
	}
	if cmd.Flags().Changed("http-proxy") {
		flags.HTTPProxy = strings.TrimSpace(flagHTTPProxy)
	}
	if cmd.Flags().Changed("https-proxy") {
		flags.HTTPSProxy = strings.TrimSpace(flagHTTPSProxy)
	}
	if cmd.Flags().Changed("no-proxy") {
		flags.NoProxy = cleanStringList(flagNoProxy)
	}
	if cmd.Flags().Changed("no-dns") {
		value := flagNoDNS
		flags.NoDNS = &value
	}
	if cmd.Flags().Changed("dns") {
		flags.DNS = cleanStringList(flagDNS)
	}
	if cmd.Flags().Changed("dns-domain") {
		flags.DNSDomain = strings.TrimSpace(flagDNSDomain)
	}
	if cmd.Flags().Changed("dns-search") {
		flags.DNSSearch = cleanStringList(flagDNSSearch)
	}
	if cmd.Flags().Changed("dns-option") {
		flags.DNSOptions = cleanStringList(flagDNSOptions)
	}
	if cmd.Flags().Changed("rm-container-on-exit") {
		value := flagRemoveContainerExit
		flags.RemoveContainerOnExit = &value
	}
	if cmd.Flags().Changed("gentic-dir") {
		flags.GenticDir = strings.TrimSpace(flagGenticDir)
	}
	if cmd.Flags().Changed("gentic-debug") {
		value := flagGenticDebug
		flags.GenticDebug = &value
	}
	if cmd.Flags().Changed("cpus") {
		flags.CPUs = strings.TrimSpace(flagCPUs)
	}
	if cmd.Flags().Changed("memory") {
		flags.Memory = strings.TrimSpace(flagMemory)
	}
	if cmd.Flags().Changed("disk") {
		flags.Disk = strings.TrimSpace(flagDisk)
	}
	if cmd.Flags().Changed("platform") {
		flags.Platform = strings.TrimSpace(flagPlatform)
	}
	if cmd.Flags().Changed("ulimit") {
		flags.Ulimits = profile.MergeUlimits(flags.Ulimits, flagUlimits)
	}
	if cmd.Flags().Changed("cap-add") {
		flags.CapAdd = cleanStringList(flagCapAdd)
	}
	if cmd.Flags().Changed("cap-drop") {
		flags.CapDrop = cleanStringList(flagCapDrop)
	}
	return flags
}

func shouldSkipRuntimeCheck(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if current.Name() == "runtime" {
			return true
		}
	}
	return false
}

func shouldSkipDependencyCheck(cmd *cobra.Command) bool {
	if cmd == nil {
		return true
	}
	if cmd.Parent() == nil || isHelpCommand(cmd) {
		return true
	}
	if shouldSkipRuntimeCheck(cmd) {
		return true
	}
	return false
}

func isHelpCommand(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		switch current.Name() {
		case "help", "completion":
			return true
		}
	}
	return false
}

func ensureDependencies(cmd *cobra.Command, flags profile.Flags) error {
	if err := runtime.CheckHostPlatform(); err != nil {
		return err
	}
	hostUser, err := runtime.ResolveHostUser()
	if err != nil {
		return err
	}
	state, err := runtime.CheckDependencies(flags)
	if err != nil {
		return err
	}
	missing := runtime.MissingRequiredDependencies(state)
	if len(missing) > 0 {
		for _, dep := range missing {
			label := dependencyInstallLabel(dep)
			if err := runWithSpinner(label, func() error {
				return runtime.InstallDependency(dep.Name, cmd.OutOrStdout())
			}); err != nil {
				finalState, _ := runtime.CheckDependencies(flags)
				_ = runtime.WriteDependenciesState(hostUser.Home, finalState)
				return err
			}
		}
		state, err = runtime.CheckDependencies(flags)
		if err != nil {
			return err
		}
	}
	if err := ensureContainerRuntimeService(state); err != nil {
		return err
	}
	if err := runtime.WriteDependenciesState(hostUser.Home, state); err != nil {
		return err
	}
	if missing := runtime.MissingRequiredDependencies(state); len(missing) > 0 {
		return fmt.Errorf("missing required dependencies: %s (see %s)", formatDependencyList(missing), profile.DependenciesPath(hostUser.Home))
	}
	return nil
}

func ensureContainerRuntimeService(state runtime.DependenciesState) error {
	bin, ok := installedDependencyPath(state, "container")
	if !ok {
		return nil
	}
	running, statusErr := runtime.ContainerRuntimeServiceRunning(bin)
	if statusErr == nil && running {
		return nil
	}
	if err := runWithSpinner("Start container runtime:", func() error {
		return runtime.StartContainerRuntimeService(bin)
	}); err != nil {
		if statusErr != nil {
			return fmt.Errorf("container service status check failed: %v; %w", statusErr, err)
		}
		return err
	}
	return nil
}

func installedDependencyPath(state runtime.DependenciesState, name string) (string, bool) {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return "", false
	}
	for _, dep := range state.Dependencies {
		if !dep.Installed {
			continue
		}
		if strings.ToLower(strings.TrimSpace(dep.Name)) != target {
			continue
		}
		path := strings.TrimSpace(dep.Path)
		if path == "" {
			return "", false
		}
		return path, true
	}
	return "", false
}

func dependencyInstallLabel(dep runtime.DependencyState) string {
	verb := "Install"
	if dep.Outdated {
		verb = "Upgrade"
	}
	name := strings.TrimSpace(dep.Name)
	switch strings.ToLower(name) {
	case "container":
		return verb + " container runtime:"
	default:
		return fmt.Sprintf("Install %s:", strings.TrimSpace(name))
	}
}

func formatDependencyList(deps []runtime.DependencyState) string {
	names := make([]string, 0, len(deps))
	for _, dep := range deps {
		name := strings.TrimSpace(dep.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "unknown"
	}
	return strings.Join(names, ", ")
}

func cleanStringList(values []string) []string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		clean = append(clean, trimmed)
	}
	return clean
}
