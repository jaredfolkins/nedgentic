package frontend

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

const (
	// desktopGuestCommand ships in the minimal image (containers/*/minimal/scripts).
	desktopGuestCommand = "/usr/local/bin/ned-desktop"
	// desktopLinePrefix starts the one machine-readable line `ned-desktop start` prints.
	desktopLinePrefix = "NED_DESKTOP "
	desktopPagePath   = "/vnc.html"
)

// Test seams.
var (
	desktopExecOutput = runtime.ContainerExecOutput
	openDesktopURL    = func(rawURL string) error {
		return exec.Command("open", rawURL).Run()
	}
)

var desktopCmd = &cobra.Command{
	Use:   "desktop [vm]",
	Short: "Open the VM's desktop (Chromium and a terminal) in your browser",
	Long:  "Start the VM's tiny desktop (Chromium and a terminal, served by noVNC on port 6080) as your user and open it in your Mac's browser. Without a vm, uses the default profile's VM.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target, err := resolveDesktopTarget(cmd, args)
		if err != nil {
			output.Printf("Desktop failed: %v", err)
			os.Exit(1)
		}
		if target.IP == "" {
			output.Printf("Desktop failed: VM %s has no IPv4 address", target.Name)
			os.Exit(1)
		}
		var stdout []byte
		if err := runWithSpinner("Start desktop: "+target.Name, func() error {
			var execErr error
			stdout, execErr = desktopExecOutput(target.Bin, target.execArgs("start"))
			return execErr
		}); err != nil {
			output.Printf("Desktop failed: %v", err)
			os.Exit(1)
		}
		port, password, err := parseDesktopLine(stdout)
		if err != nil {
			output.Printf("Desktop failed: %v", err)
			os.Exit(1)
		}
		output.Printf("Desktop: %s", desktopURL(target.IP, port, ""))
		output.Printf("Hint: agents in the VM use DISPLAY=:1; run `ned desktop stop %s` to stop it.", target.Name)
		if err := openDesktopURL(desktopURL(target.IP, port, password)); err != nil {
			output.Printf("Desktop failed: open browser: %v", err)
			os.Exit(1)
		}
	},
}

var desktopStopCmd = &cobra.Command{
	Use:   "stop [vm]",
	Short: "Stop the VM's desktop",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target, err := resolveDesktopTarget(cmd, args)
		if err != nil {
			output.Printf("Desktop stop failed: %v", err)
			os.Exit(1)
		}
		if err := runWithSpinner("Stop desktop: "+target.Name, func() error {
			_, execErr := desktopExecOutput(target.Bin, target.execArgs("stop"))
			return execErr
		}); err != nil {
			output.Printf("Desktop stop failed: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	desktopCmd.AddCommand(desktopStopCmd)
	rootCmd.AddCommand(desktopCmd)
}

// desktopTarget is a running VM plus how to exec in it as the host user.
type desktopTarget struct {
	Bin     string
	Name    string
	IP      string
	Workdir string
	Env     []string
	UID     int
	GID     int
}

func (t desktopTarget) execArgs(action string) []string {
	uid, gid := t.UID, t.GID
	// BuildExecArgs only fails without a name or command; both are set.
	args, _ := runtime.BuildExecArgs(runtime.ExecOptions{
		UID:           &uid,
		GID:           &gid,
		Env:           t.Env,
		Workdir:       t.Workdir,
		ContainerName: t.Name,
		Command:       []string{desktopGuestCommand, action},
	})
	return args
}

func resolveDesktopTarget(cmd *cobra.Command, args []string) (desktopTarget, error) {
	name, err := desktopVMName(args)
	if err != nil {
		return desktopTarget{}, err
	}
	flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
	logging.SetLevel(logging.ParseLevel(flags.LogLevel))
	bin, err := runtime.ResolveContainerBin(flags)
	if err != nil {
		return desktopTarget{}, err
	}
	entries, err := runtime.ListContainers(bin)
	if err != nil {
		return desktopTarget{}, err
	}
	entry, ok := findContainerEntry(entries, name)
	if !ok {
		return desktopTarget{}, fmt.Errorf("VM %s not found; start it with `ned gentic personal <profile> %s`", name, name)
	}
	if !isRunningStatus(entry.Status) {
		return desktopTarget{}, fmt.Errorf("VM %s is %s; start it with `ned gentic` first", name, fieldOrDashText(entry.Status))
	}
	prof, err := profileForEntry(entry)
	if err != nil {
		return desktopTarget{}, fmt.Errorf("resolve profile: %w", err)
	}
	if prof != nil {
		flags = applyFlagOverrides(runtime.EffectiveFlags(prof), cmd)
	}
	shellPath, err := runtime.ResolveShellPath(prof)
	if err != nil {
		return desktopTarget{}, err
	}
	hostUser, err := runtime.ResolveHostUser()
	if err != nil {
		return desktopTarget{}, err
	}
	workdir := strings.TrimSpace(entry.Configuration.Labels[runtime.LabelWorkspaceKey])
	if workdir == "" {
		workdir = runtime.GuestHomeForUser(hostUser.Name)
	}
	return desktopTarget{
		Bin:     bin,
		Name:    name,
		IP:      containerIPv4(entry),
		Workdir: workdir,
		Env:     runtime.GuestUserEnv(hostUser, shellPath, flags),
		UID:     hostUser.UID,
		GID:     hostUser.GID,
	}, nil
}

// desktopVMName returns the named VM, or the VM `ned gentic personal default` creates.
func desktopVMName(args []string) (string, error) {
	if len(args) > 0 {
		return normalizeAgentName("vm name", args[0])
	}
	return runtime.ContainerNameForProfile(profile.DefaultProfileName)
}

func findContainerEntry(entries []runtime.ContainerListEntry, name string) (runtime.ContainerListEntry, bool) {
	for _, entry := range entries {
		if entry.Configuration.ID == name {
			return entry, true
		}
	}
	return runtime.ContainerListEntry{}, false
}

// containerIPv4 returns the first network's IPv4 address without its prefix length.
func containerIPv4(entry runtime.ContainerListEntry) string {
	for _, network := range entry.Networks {
		raw := strings.TrimSpace(network.IPv4Address)
		if before, _, ok := strings.Cut(raw, "/"); ok {
			raw = before
		}
		if ip := net.ParseIP(raw); ip != nil && ip.To4() != nil {
			return ip.String()
		}
	}
	return ""
}

// parseDesktopLine reads `NED_DESKTOP port=<port> password=<password>` from
// ned-desktop output. Errors never echo the password.
func parseDesktopLine(out []byte) (int, string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, desktopLinePrefix) {
			continue
		}
		fields := map[string]string{}
		for _, field := range strings.Fields(strings.TrimPrefix(line, desktopLinePrefix)) {
			if key, value, ok := strings.Cut(field, "="); ok {
				fields[key] = value
			}
		}
		port, err := strconv.Atoi(fields["port"])
		if err != nil || port < 1 || port > 65535 {
			return 0, "", fmt.Errorf("ned-desktop reported an invalid port")
		}
		if fields["password"] == "" {
			return 0, "", fmt.Errorf("ned-desktop reported no password")
		}
		return port, fields["password"], nil
	}
	return 0, "", fmt.Errorf("ned-desktop gave no connection details; is the VM using the minimal image?")
}

// desktopURL builds the noVNC page URL; an empty password leaves it out for display.
func desktopURL(ip string, port int, password string) string {
	query := url.Values{}
	query.Set("autoconnect", "1")
	// x11vnc cannot resize Xvfb on request, so the page scales the screen instead.
	query.Set("resize", "scale")
	if password != "" {
		query.Set("password", password)
	}
	page := url.URL{
		Scheme:   "http",
		Host:     net.JoinHostPort(ip, strconv.Itoa(port)),
		Path:     desktopPagePath,
		RawQuery: query.Encode(),
	}
	return page.String()
}
