package profile

import "strings"

type ImportsConfig struct {
	Binaries    []string         `json:"binaries"`
	Files       []FileMount      `json:"files"`
	Directories []DirectoryMount `json:"directories"`
}

func ImportsFromFields(binaries []string, files []FileMount, directories []DirectoryMount) *ImportsConfig {
	config := &ImportsConfig{
		Binaries:    cleanTemplateStringList(binaries),
		Files:       append([]FileMount(nil), files...),
		Directories: append([]DirectoryMount(nil), directories...),
	}
	if config.IsZero() {
		return nil
	}
	return config
}

func (config *ImportsConfig) IsZero() bool {
	if config == nil {
		return true
	}
	return len(cleanTemplateStringList(config.Binaries)) == 0 &&
		len(config.Files) == 0 &&
		len(config.Directories) == 0
}

func TrimStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	copied := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			copied[key] = value
		}
	}
	return copied
}
