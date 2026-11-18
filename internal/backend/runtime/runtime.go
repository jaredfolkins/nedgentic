// Package runtime re-exports the internal/runtime API as package-level
// variables so frontend tests can substitute individual operations.
package runtime

import r "nedgentic/internal/runtime"

type ContainerListEntry = r.ContainerListEntry
type ContainerListNetwork = r.ContainerListNetwork
type ExecOptions = r.ExecOptions
type SessionLock = r.SessionLock
type ContainerMount = r.ContainerMount
type MountPlan = r.MountPlan
type RunContainerOptions = r.RunContainerOptions
type ExecShellOptions = r.ExecShellOptions
type Profile = r.Profile
type RuntimeActionOptions = r.RuntimeActionOptions
type HostUser = r.HostUser
type ImageListEntry = r.ImageListEntry
type DependencyState = r.DependencyState
type DependenciesState = r.DependenciesState
type VolumeEntry = r.VolumeEntry
type RuntimeStatus = r.RuntimeStatus
type SystemDiskUsage = r.SystemDiskUsage
type SystemDiskUsageEntry = r.SystemDiskUsageEntry
type PathUsage = r.PathUsage
type FilesystemUsage = r.FilesystemUsage

type StepTimer = r.StepTimer

type FileCopySpec = r.FileCopySpec

const (
	ContainerLabelKey = r.ContainerLabelKey
	LabelProfileKey   = r.LabelProfileKey
	LabelHostnameKey  = r.LabelHostnameKey
	LabelWorkspaceKey = r.LabelWorkspaceKey
	LabelUserKey      = r.LabelUserKey
	LabelImageKey     = r.LabelImageKey
	GenticMountDir    = r.GenticMountDir
)

var (
	ResolveContainerBin            = r.ResolveContainerBin
	CheckContainerRuntime          = r.CheckContainerRuntime
	ListContainers                 = r.ListContainers
	ContainerStatusByName          = r.ContainerStatusByName
	ContainerStart                 = r.ContainerStart
	ContainerStop                  = r.ContainerStop
	ContainerDelete                = r.ContainerDelete
	DeleteAgentAuthVolume          = r.DeleteAgentAuthVolume
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
	AppendMount                    = r.AppendMount
	EnsureStagingDir               = r.EnsureStagingDir
	EnsureRunnerDir                = r.EnsureRunnerDir
	EnsureGenticScripts            = r.EnsureGenticScripts
	AcquireSessionLock             = r.AcquireSessionLock
	RemoveContainerIfIdle          = r.RemoveContainerIfIdle
	ApplyHostBinaries              = r.ApplyHostBinaries
	GenticScriptsPresent           = r.GenticScriptsPresent
	ContainerNameForProfile        = r.ContainerNameForProfile
	LoadProfile                    = r.LoadProfile
	ParseProfileConfig             = r.ParseProfileConfig
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
	ListImages                     = r.ListImages
	ContainerImageExists           = r.ContainerImageExists
	ImagePrune                     = r.ImagePrune
	ContainerPrune                 = r.ContainerPrune
	ManagedPull                    = r.ManagedPull
	FetchReleaseImage              = r.FetchReleaseImage
	ContainerExport                = r.ContainerExport
	ContainerSystemStatus          = r.ContainerSystemStatus
	ContainerImageLoad             = r.ContainerImageLoad
	ContainerImageDelete           = r.ContainerImageDelete
	InstallContainerRuntime        = r.InstallContainerRuntime
	RemoveContainerRuntime         = r.RemoveContainerRuntime
	StartContainerRuntime          = r.StartContainerRuntime
	StopContainerRuntime           = r.StopContainerRuntime
	ContainerRuntimeServiceRunning = r.ContainerRuntimeServiceRunning
	StartContainerRuntimeService   = r.StartContainerRuntimeService
	EnsureContainerRuntimeStarted  = r.EnsureContainerRuntimeStarted
	WriteRuntimeStatus             = r.WriteRuntimeStatus
	CollectRuntimeStatus           = r.CollectRuntimeStatus
	ListStagingDirs                = r.ListStagingDirs
	RemoveStagingDir               = r.RemoveStagingDir
	CleanStagingAll                = r.CleanStagingAll
	NewStepTimer                   = r.NewStepTimer
	FormatBytes                    = r.FormatBytes
	ListVolumes                    = r.ListVolumes
	BuilderDelete                  = r.BuilderDelete
	FetchSystemDiskUsage           = r.FetchSystemDiskUsage
	FetchContainerDataRoot         = r.FetchContainerDataRoot
	FetchPathUsage                 = r.FetchPathUsage
	FetchFilesystemUsage           = r.FetchFilesystemUsage
)
