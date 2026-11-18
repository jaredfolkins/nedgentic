package runtime

import "nedgentic/internal/profile"

type RunContainerOptions struct {
	Bin              string
	Image            string
	ContainerName    string
	ProfileName      string
	Hostname         string
	Flags            profile.Flags
	Plan             *MountPlan
	HostUser         *HostUser
	ShellPath        string
	DesiredWorkdir   string
	ContainerWorkdir string
	RunnerDir        string
	Detached         bool
	KeepAlive        bool
}

type BuildRunArgsOptions struct {
	Image            string
	ContainerName    string
	ProfileName      string
	Hostname         string
	Flags            profile.Flags
	Plan             *MountPlan
	HostUser         *HostUser
	DesiredWorkdir   string
	ContainerWorkdir string
	Detached         bool
	TTY              bool
	CPU              string
	Memory           string
	Ulimits          []string
	AgentAuth        bool
}

type ExecShellOptions struct {
	Bin           string
	ContainerName string
	ShellPath     string
	Workdir       string
	Flags         profile.Flags
	HostUser      *HostUser
	Ulimits       []string
}
