// Package runtime re-exports the internal/runtime API as package-level
// variables so frontend tests can substitute individual operations.
package runtime

import r "nedgentic/internal/runtime"

type ContainerListEntry = r.ContainerListEntry
type ContainerListNetwork = r.ContainerListNetwork
type ExecOptions = r.ExecOptions
type SessionLock = r.SessionLock
type MountPlan = r.MountPlan
type RunContainerOptions = r.RunContainerOptions
type ExecShellOptions = r.ExecShellOptions
type Profile = r.Profile
type HostUser = r.HostUser
type DependencyState = r.DependencyState
type DependenciesState = r.DependenciesState
type SystemDiskUsageEntry = r.SystemDiskUsageEntry

const (
	ContainerLabelKey = r.ContainerLabelKey
	LabelProfileKey   = r.LabelProfileKey
	LabelHostnameKey  = r.LabelHostnameKey
	LabelWorkspaceKey = r.LabelWorkspaceKey
	LabelUserKey      = r.LabelUserKey
	LabelImageKey     = r.LabelImageKey
)

var (
	ResolveContainerBin            = r.ResolveContainerBin
	ListContainers                 = r.ListContainers
	ContainerStatusByName          = r.ContainerStatusByName
	ContainerStart                 = r.ContainerStart
	ContainerStop                  = r.ContainerStop
	ContainerDelete                = r.ContainerDelete
	DeleteAgentAuthVolume          = r.DeleteAgentAuthVolume
	RemoveStagingDir               = r.RemoveStagingDir
	ContainerLogs                  = r.ContainerLogs
	RunContainerExec               = r.RunContainerExec
	ContainerExecOutput            = r.ContainerExecOutput
	GuestUserEnv                   = r.GuestUserEnv
	BuildExecArgs                  = r.BuildExecArgs
	WantsTTY                       = r.WantsTTY
	ExecContainerShell             = r.ExecContainerShell
	RunContainer                   = r.RunContainer
	WaitForContainerReady          = r.WaitForContainerReady
	WaitForContainerSystemdReady   = r.WaitForContainerSystemdReady
	BootstrapContainerSystemd      = r.BootstrapContainerSystemd
	ResolveHostUser                = r.ResolveHostUser
	ResolveShellPath               = r.ResolveShellPath
	ResolveRunningWorkdir          = r.ResolveRunningWorkdir
	MapHostPathToGuest             = r.MapHostPathToGuest
	GuestHomeForUser               = r.GuestHomeForUser
	ResolveTargetPath              = r.ResolveTargetPath
	PlanMounts                     = r.PlanMounts
	EnsureStagingDir               = r.EnsureStagingDir
	EnsureRunnerDir                = r.EnsureRunnerDir
	AcquireSessionLock             = r.AcquireSessionLock
	RemoveContainerIfIdle          = r.RemoveContainerIfIdle
	ApplyHostBinaries              = r.ApplyHostBinaries
	ContainerNameForProfile        = r.ContainerNameForProfile
	LoadProfile                    = r.LoadProfile
	EffectiveFlags                 = r.EffectiveFlags
	ResolveRunImage                = r.ResolveRunImage
	WriteProfileTemplate           = r.WriteProfileTemplate
	WriteProfileConfig             = r.WriteProfileConfig
	ProxyEnvEntries                = r.ProxyEnvEntries
	CheckDependencies              = r.CheckDependencies
	MissingRequiredDependencies    = r.MissingRequiredDependencies
	CheckHostPlatform              = r.CheckHostPlatform
	InstallDependency              = r.InstallDependency
	WriteDependenciesState         = r.WriteDependenciesState
	ContainerImageExists           = r.ContainerImageExists
	ManagedPull                    = r.ManagedPull
	FetchReleaseImage              = r.FetchReleaseImage
	ContainerRuntimeServiceRunning = r.ContainerRuntimeServiceRunning
	StartContainerRuntimeService   = r.StartContainerRuntimeService
	NewStepTimer                   = r.NewStepTimer
	FormatBytes                    = r.FormatBytes
	FetchSystemDiskUsage           = r.FetchSystemDiskUsage
	FetchContainerDataRoot         = r.FetchContainerDataRoot
	FetchPathUsage                 = r.FetchPathUsage
	FetchFilesystemUsage           = r.FetchFilesystemUsage
)
