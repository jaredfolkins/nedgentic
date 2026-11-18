package runtime

import (
	"time"

	"nedgentic/internal/profile"
)

const (
	DefaultShellPath    = profile.DefaultShellPath
	SecretsMountPath    = "/mnt/ned-secrets"
	GenticMountDir      = "/mnt/ned-gentic"
	RunnerMountDir      = "/mnt/ned-runner"
	WorkspaceMountDir   = "/mnt/ned-workspaces"
	DiskMountDir        = "/mnt/ned-disk"
	AgentAuthMountDir   = "/mnt/ned-auth"
	AgentAuthVolumeSize = "1G"
	RunnerScriptName    = "runner.sh"
	BootstrapScriptName = "bootstrap.sh"
	ContainerReadyFile  = "/tmp/ned-ready"

	containerReadyPollInterval = 250 * time.Millisecond
	containerReadyTimeout      = 60 * time.Second
	MountTypeBind              = "bind"
	MountTypeVolume            = "volume"
	BuilderRetryCount          = 3
	BuilderRetryBase           = 500 * time.Millisecond
	BuilderRetryStep           = 3
	ContainerLabelKey          = "nedgentic"
	LabelProfileKey            = "ned.profile"
	LabelImageKey              = "ned.image"
	LabelUserKey               = "ned.user"
	LabelWorkspaceKey          = "ned.workspace"
	LabelHostnameKey           = "ned.hostname"
)
