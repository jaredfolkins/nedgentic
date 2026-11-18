package profile

type TemplateOptions struct {
	Shell       string
	Flags       Flags
	Resources   *ResourcesConfig
	Imports     *ImportsConfig
	Binaries    []string
	Files       []FileMount
	Directories []DirectoryMount
	Image       *ImageState
}
