package runtime

import (
	"os"

	"nedgentic/internal/profile"
)

type Profile struct {
	Shell       string                   `json:"shell"`
	Flags       *profile.Flags           `json:"flags"`
	Resources   *profile.ResourcesConfig `json:"resources"`
	Imports     *profile.ImportsConfig   `json:"imports"`
	Image       *profile.ImageState      `json:"image"`
	Binaries    []string                 `json:"binaries"`
	Files       []profile.FileMount      `json:"files"`
	Directories []profile.DirectoryMount `json:"directories"`
}

type ContainerMount struct {
	Type     string
	Source   string
	Target   string
	ReadOnly bool
}

type FileCopySpec struct {
	SourceRel string
	DestPath  string
	Mode      os.FileMode
	IsDir     bool
}

type MountPlan struct {
	Mounts         []ContainerMount
	CopySpecs      []FileCopySpec
	StagingRoot    string
	WorkspacePath  string
	WorkspaceMount string
}

type HostUser struct {
	Name string
	UID  int
	GID  int
	Home string
}
