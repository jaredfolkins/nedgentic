package profile

import "strings"

type ResourcesConfig struct {
	CPUs   string `json:"cpus"`
	Memory string `json:"memory"`
	Disk   string `json:"disk"`
}

func ResourcesFromFlags(flags Flags) *ResourcesConfig {
	resources := &ResourcesConfig{
		CPUs:   strings.TrimSpace(flags.CPUs),
		Memory: strings.TrimSpace(flags.Memory),
		Disk:   strings.TrimSpace(flags.Disk),
	}
	if resources.IsZero() {
		return nil
	}
	return resources
}

func (resources *ResourcesConfig) IsZero() bool {
	if resources == nil {
		return true
	}
	return strings.TrimSpace(resources.CPUs) == "" &&
		strings.TrimSpace(resources.Memory) == "" &&
		strings.TrimSpace(resources.Disk) == ""
}

func (resources *ResourcesConfig) Flags() Flags {
	flags := Flags{}
	if resources == nil {
		return flags
	}
	flags.CPUs = strings.TrimSpace(resources.CPUs)
	flags.Memory = strings.TrimSpace(resources.Memory)
	flags.Disk = strings.TrimSpace(resources.Disk)
	return flags
}
