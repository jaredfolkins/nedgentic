package runtime

import (
	"path/filepath"
	"testing"

	"nedgentic/internal/profile"
)

func TestBuildRunArgsDetachedSuppressesContainerProgress(t *testing.T) {
	flags := profile.DefaultFlags()

	args := buildRunArgs(BuildRunArgsOptions{
		Image:          "docker.io/library/alpine",
		ContainerName:  "ned-test",
		ProfileName:    "test",
		Hostname:       "ned-test",
		Flags:          flags,
		Plan:           &MountPlan{},
		HostUser:       &HostUser{Name: "tester"},
		Detached:       true,
		TTY:            true,
		DesiredWorkdir: "",
	})

	if !containsArgPair(args, "--progress", "none") {
		t.Fatalf("expected detached run to suppress container progress, got %#v", args)
	}
	if !containsArg(args, "-d") {
		t.Fatalf("expected detached run arg, got %#v", args)
	}
	if containsArg(args, "-it") {
		t.Fatalf("did not expect interactive run args for detached start, got %#v", args)
	}
}

func TestBuildRunArgsInitFlags(t *testing.T) {
	flags := profile.DefaultFlags()
	enabled := true
	flags.Init = &enabled
	flags.InitImage = "docker.io/library/custom-init:latest"

	args := buildRunArgs(BuildRunArgsOptions{
		Image:          "docker.io/library/alpine",
		ContainerName:  "ned-test",
		ProfileName:    "test",
		Hostname:       "ned-test",
		Flags:          flags,
		Plan:           &MountPlan{},
		HostUser:       &HostUser{Name: "tester"},
		Detached:       false,
		TTY:            false,
		DesiredWorkdir: "",
	})

	if !containsArg(args, "--init") {
		t.Fatalf("expected --init in args, got %#v", args)
	}
	if !containsArgPair(args, "--init-image", "docker.io/library/custom-init:latest") {
		t.Fatalf("expected --init-image in args, got %#v", args)
	}
}

func TestBuildRunArgsSystemdUsesInitAsEntrypoint(t *testing.T) {
	flags := profile.DefaultFlags()
	enabled := true
	flags.Systemd = &enabled
	args := buildRunArgs(BuildRunArgsOptions{
		Image:          "docker.io/library/alpine",
		ContainerName:  "ned-test",
		ProfileName:    "test",
		Hostname:       "ned-test",
		Flags:          flags,
		Plan:           &MountPlan{},
		HostUser:       &HostUser{Name: "tester"},
		Detached:       false,
		TTY:            false,
		DesiredWorkdir: "",
	})

	if !containsArgPair(args, "--entrypoint", "/sbin/init") {
		t.Fatalf("expected /sbin/init entrypoint, got %#v", args)
	}
	if containsArgPair(args, "--entrypoint", "/bin/sh") {
		t.Fatalf("did not expect /bin/sh entrypoint, got %#v", args)
	}
	if containsArgPair(args, "--init-image", "docker.io/library/custom-init:latest") {
		t.Fatalf("did not expect init-image in systemd mode, got %#v", args)
	}
	if containsArg(args, filepath.Join(RunnerMountDir, RunnerScriptName)) {
		t.Fatalf("did not expect runner script arg in systemd mode, got %#v", args)
	}
}

func TestBuildRunArgsResourceValues(t *testing.T) {
	flags := profile.DefaultFlags()
	args := buildRunArgs(BuildRunArgsOptions{
		Image:          "docker.io/library/alpine",
		ContainerName:  "ned-test",
		ProfileName:    "test",
		Hostname:       "ned-test",
		Flags:          flags,
		Plan:           &MountPlan{},
		HostUser:       &HostUser{Name: "tester"},
		Detached:       false,
		TTY:            false,
		DesiredWorkdir: "",
		CPU:            "2",
		Memory:         "1G",
	})

	if !containsArgPair(args, "--cpus", "2") {
		t.Fatalf("expected --cpus in args, got %#v", args)
	}
	if !containsArgPair(args, "--memory", "1G") {
		t.Fatalf("expected --memory in args, got %#v", args)
	}
}

func TestBuildRunArgsUlimits(t *testing.T) {
	flags := profile.DefaultFlags()
	args := buildRunArgs(BuildRunArgsOptions{
		Image:          "docker.io/library/alpine",
		ContainerName:  "ned-test",
		ProfileName:    "test",
		Hostname:       "ned-test",
		Flags:          flags,
		Plan:           &MountPlan{},
		HostUser:       &HostUser{Name: "tester"},
		Detached:       false,
		TTY:            false,
		DesiredWorkdir: "",
		Ulimits:        []string{"nofile=1024:2048", "nproc=512"},
	})

	if !containsArgPair(args, "--ulimit", "nofile=1024:2048") {
		t.Fatalf("expected nofile ulimit in args, got %#v", args)
	}
	if !containsArgPair(args, "--ulimit", "nproc=512") {
		t.Fatalf("expected nproc ulimit in args, got %#v", args)
	}
}

func TestBuildRunArgsCapabilities(t *testing.T) {
	flags := profile.DefaultFlags()
	flags.CapAdd = []string{"NET_ADMIN", "CAP_SYS_PTRACE"}
	flags.CapDrop = []string{"MKNOD"}

	args := buildRunArgs(BuildRunArgsOptions{
		Image:          "docker.io/library/alpine",
		ContainerName:  "ned-test",
		ProfileName:    "test",
		Hostname:       "ned-test",
		Flags:          flags,
		Plan:           &MountPlan{},
		HostUser:       &HostUser{Name: "tester"},
		Detached:       false,
		TTY:            false,
		DesiredWorkdir: "",
	})

	if !containsArgPair(args, "--cap-add", "NET_ADMIN") {
		t.Fatalf("expected NET_ADMIN cap-add in args, got %#v", args)
	}
	if !containsArgPair(args, "--cap-add", "CAP_SYS_PTRACE") {
		t.Fatalf("expected CAP_SYS_PTRACE cap-add in args, got %#v", args)
	}
	if !containsArgPair(args, "--cap-drop", "MKNOD") {
		t.Fatalf("expected MKNOD cap-drop in args, got %#v", args)
	}
}

func TestBuildRunArgsNetworking(t *testing.T) {
	flags := profile.DefaultFlags()
	flags.Network = "ned-lab"
	noDNS := true
	flags.NoDNS = &noDNS
	flags.DNS = []string{"203.0.113.53", "198.51.100.2"}
	flags.DNSDomain = "example.test"
	flags.DNSSearch = []string{"svc.local", "corp.local"}
	flags.DNSOptions = []string{"ndots:1", "timeout:2"}

	args := buildRunArgs(BuildRunArgsOptions{
		Image:          "docker.io/library/alpine",
		ContainerName:  "ned-test",
		ProfileName:    "test",
		Hostname:       "ned-test",
		Flags:          flags,
		Plan:           &MountPlan{},
		HostUser:       &HostUser{Name: "tester"},
		Detached:       false,
		TTY:            false,
		DesiredWorkdir: "",
	})

	if !containsArgPair(args, "--network", "ned-lab") {
		t.Fatalf("expected --network in args, got %#v", args)
	}
	if !containsArg(args, "--no-dns") {
		t.Fatalf("expected --no-dns in args, got %#v", args)
	}
	if !containsArgPair(args, "--dns", "203.0.113.53") || !containsArgPair(args, "--dns", "198.51.100.2") {
		t.Fatalf("expected --dns values in args, got %#v", args)
	}
	if !containsArgPair(args, "--dns-domain", "example.test") {
		t.Fatalf("expected --dns-domain in args, got %#v", args)
	}
	if !containsArgPair(args, "--dns-search", "svc.local") || !containsArgPair(args, "--dns-search", "corp.local") {
		t.Fatalf("expected --dns-search values in args, got %#v", args)
	}
	if !containsArgPair(args, "--dns-option", "ndots:1") || !containsArgPair(args, "--dns-option", "timeout:2") {
		t.Fatalf("expected --dns-option values in args, got %#v", args)
	}
}

func TestBuildRunArgsProxyEnv(t *testing.T) {
	flags := profile.DefaultFlags()
	flags.Proxy = "http://proxy.base:8080"
	flags.NoProxy = []string{"localhost", "127.0.0.1"}

	args := buildRunArgs(BuildRunArgsOptions{
		Image:          "docker.io/library/alpine",
		ContainerName:  "ned-test",
		ProfileName:    "test",
		Hostname:       "ned-test",
		Flags:          flags,
		Plan:           &MountPlan{},
		HostUser:       &HostUser{Name: "tester"},
		Detached:       false,
		TTY:            false,
		DesiredWorkdir: "",
	})

	for _, pair := range [][2]string{
		{"--env", "HTTP_PROXY=http://proxy.base:8080"},
		{"--env", "http_proxy=http://proxy.base:8080"},
		{"--env", "HTTPS_PROXY=http://proxy.base:8080"},
		{"--env", "https_proxy=http://proxy.base:8080"},
		{"--env", "NO_PROXY=localhost,127.0.0.1"},
		{"--env", "no_proxy=localhost,127.0.0.1"},
	} {
		if !containsArgPair(args, pair[0], pair[1]) {
			t.Fatalf("expected %s %q in args, got %#v", pair[0], pair[1], args)
		}
	}
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}

func containsArgPair(args []string, key, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}
