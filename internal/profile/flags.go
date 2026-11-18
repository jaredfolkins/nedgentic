package profile

import "strings"

type Flags struct {
	LogLevel              string   `json:"log-level"`
	ContainerBin          string   `json:"container-bin"`
	Init                  *bool    `json:"init"`
	InitImage             string   `json:"init-image"`
	Systemd               *bool    `json:"systemd"`
	Network               string   `json:"network"`
	Proxy                 string   `json:"proxy"`
	HTTPProxy             string   `json:"http-proxy"`
	HTTPSProxy            string   `json:"https-proxy"`
	NoProxy               []string `json:"no-proxy"`
	NoDNS                 *bool    `json:"no-dns"`
	DNS                   []string `json:"dns"`
	DNSDomain             string   `json:"dns-domain"`
	DNSSearch             []string `json:"dns-search"`
	DNSOptions            []string `json:"dns-option"`
	RemoveContainerOnExit *bool    `json:"rm-container-on-exit"`
	CPUs                  string   `json:"cpus"`
	Memory                string   `json:"memory"`
	Disk                  string   `json:"disk"`
	Platform              string   `json:"platform"`
	Ulimits               []string `json:"ulimits"`
	CapAdd                []string `json:"cap-add"`
	CapDrop               []string `json:"cap-drop"`
}

func DefaultFlags() Flags {
	init := false
	systemd := false
	noDNS := false
	remove := false
	return Flags{
		LogLevel:              "info",
		ContainerBin:          "container",
		Init:                  &init,
		InitImage:             "",
		Systemd:               &systemd,
		Network:               "",
		Proxy:                 "",
		HTTPProxy:             "",
		HTTPSProxy:            "",
		NoProxy:               []string{},
		NoDNS:                 &noDNS,
		DNS:                   []string{},
		DNSDomain:             "",
		DNSSearch:             []string{},
		DNSOptions:            []string{},
		RemoveContainerOnExit: &remove,
		CPUs:                  "",
		Memory:                "",
		Disk:                  "",
		Platform:              "",
		Ulimits:               []string{},
		CapAdd:                []string{},
		CapDrop:               []string{},
	}
}

func ulimitKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if before, _, ok := strings.Cut(trimmed, "="); ok {
		return strings.ToLower(strings.TrimSpace(before))
	}
	return strings.ToLower(trimmed)
}

// MergeUlimits overlays override values onto base values by ulimit type while
// preserving stable argument order. Existing entries keep their position.
func MergeUlimits(base, override []string) []string {
	merged := make([]string, 0, len(base)+len(override))
	indexByKey := map[string]int{}
	appendOrReplace := func(values []string) {
		for _, raw := range values {
			value := strings.TrimSpace(raw)
			if value == "" {
				continue
			}
			key := ulimitKey(value)
			if key == "" {
				continue
			}
			if idx, ok := indexByKey[key]; ok {
				merged[idx] = value
				continue
			}
			indexByKey[key] = len(merged)
			merged = append(merged, value)
		}
	}
	appendOrReplace(base)
	appendOrReplace(override)
	return merged
}

func MergeFlags(base, override Flags) Flags {
	result := base
	if override.LogLevel != "" {
		result.LogLevel = override.LogLevel
	}
	if override.ContainerBin != "" {
		result.ContainerBin = override.ContainerBin
	}
	if override.Init != nil {
		result.Init = override.Init
	}
	if strings.TrimSpace(override.InitImage) != "" {
		result.InitImage = override.InitImage
	}
	if override.Systemd != nil {
		result.Systemd = override.Systemd
	}
	if strings.TrimSpace(override.Network) != "" {
		result.Network = override.Network
	}
	if strings.TrimSpace(override.Proxy) != "" {
		result.Proxy = override.Proxy
	}
	if strings.TrimSpace(override.HTTPProxy) != "" {
		result.HTTPProxy = override.HTTPProxy
	}
	if strings.TrimSpace(override.HTTPSProxy) != "" {
		result.HTTPSProxy = override.HTTPSProxy
	}
	if len(override.NoProxy) > 0 {
		result.NoProxy = append([]string(nil), override.NoProxy...)
	}
	if override.NoDNS != nil {
		result.NoDNS = override.NoDNS
	}
	if len(override.DNS) > 0 {
		result.DNS = append([]string(nil), override.DNS...)
	}
	if strings.TrimSpace(override.DNSDomain) != "" {
		result.DNSDomain = override.DNSDomain
	}
	if len(override.DNSSearch) > 0 {
		result.DNSSearch = append([]string(nil), override.DNSSearch...)
	}
	if len(override.DNSOptions) > 0 {
		result.DNSOptions = append([]string(nil), override.DNSOptions...)
	}
	if override.RemoveContainerOnExit != nil {
		result.RemoveContainerOnExit = override.RemoveContainerOnExit
	}
	if strings.TrimSpace(override.CPUs) != "" {
		result.CPUs = override.CPUs
	}
	if strings.TrimSpace(override.Memory) != "" {
		result.Memory = override.Memory
	}
	if strings.TrimSpace(override.Disk) != "" {
		result.Disk = override.Disk
	}
	if strings.TrimSpace(override.Platform) != "" {
		result.Platform = override.Platform
	}
	if len(override.Ulimits) > 0 {
		result.Ulimits = MergeUlimits(result.Ulimits, override.Ulimits)
	}
	if len(override.CapAdd) > 0 {
		result.CapAdd = append([]string(nil), override.CapAdd...)
	}
	if len(override.CapDrop) > 0 {
		result.CapDrop = append([]string(nil), override.CapDrop...)
	}
	return result
}

func FlagsEmpty(flags Flags) bool {
	return flags.LogLevel == "" &&
		flags.ContainerBin == "" &&
		flags.Init == nil &&
		strings.TrimSpace(flags.InitImage) == "" &&
		flags.Systemd == nil &&
		strings.TrimSpace(flags.Network) == "" &&
		strings.TrimSpace(flags.Proxy) == "" &&
		strings.TrimSpace(flags.HTTPProxy) == "" &&
		strings.TrimSpace(flags.HTTPSProxy) == "" &&
		len(flags.NoProxy) == 0 &&
		flags.NoDNS == nil &&
		len(flags.DNS) == 0 &&
		strings.TrimSpace(flags.DNSDomain) == "" &&
		len(flags.DNSSearch) == 0 &&
		len(flags.DNSOptions) == 0 &&
		flags.RemoveContainerOnExit == nil &&
		strings.TrimSpace(flags.CPUs) == "" &&
		strings.TrimSpace(flags.Memory) == "" &&
		strings.TrimSpace(flags.Disk) == "" &&
		strings.TrimSpace(flags.Platform) == "" &&
		len(flags.Ulimits) == 0 &&
		len(flags.CapAdd) == 0 &&
		len(flags.CapDrop) == 0
}
