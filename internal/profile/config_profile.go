package profile

type ConfigProfile struct {
	Shell     string           `json:"shell"`
	Flags     *Flags           `json:"flags"`
	Resources *ResourcesConfig `json:"resources"`
	Imports   *ImportsConfig   `json:"imports"`
	Image     *ImageState      `json:"image"`
}
