package runtime

import (
	"fmt"
	"strings"

	"nedgentic/internal/logging"
)

func ContainerLabelsByName(bin, name string) (map[string]string, error) {
	entries, err := ListContainers(bin)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Configuration.ID == name {
			if entry.Configuration.Labels == nil {
				return map[string]string{}, nil
			}
			labels := make(map[string]string, len(entry.Configuration.Labels))
			for key, value := range entry.Configuration.Labels {
				labels[key] = value
			}
			return labels, nil
		}
	}
	return nil, fmt.Errorf("container %s not found", name)
}

func ResolveRunningWorkdir(bin, containerName string, hostUser *HostUser) (string, error) {
	if hostUser == nil {
		return "", fmt.Errorf("host user is required")
	}
	labels, err := ContainerLabelsByName(bin, containerName)
	if err != nil {
		logging.Warnf("failed to read container labels for %s: %v", containerName, err)
		return GuestHomeForUser(hostUser.Name), nil
	}
	workdir := strings.TrimSpace(labels[LabelWorkspaceKey])
	if workdir != "" {
		return workdir, nil
	}
	return GuestHomeForUser(hostUser.Name), nil
}
