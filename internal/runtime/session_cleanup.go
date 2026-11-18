package runtime

import (
	"fmt"
	"strings"
)

func RemoveContainerIfIdle(bin, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("container name is required")
	}
	exists, status, err := ContainerStatusByName(bin, name)
	if err != nil || !exists {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(status), "running") {
		if err := ContainerStop(bin, name); err != nil {
			return err
		}
	}
	if err := ContainerDelete(bin, name); err != nil {
		exists, _, lookupErr := ContainerStatusByName(bin, name)
		if lookupErr == nil && !exists {
			return nil
		}
		return err
	}
	return nil
}
