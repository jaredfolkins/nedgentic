package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func duBytes(path string) (int64, error) {
	if strings.TrimSpace(path) == "" {
		return 0, fmt.Errorf("path is required")
	}
	cmd := exec.Command("du", "-sk", path)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return 0, fmt.Errorf("unexpected du output: %s", strings.TrimSpace(string(out)))
	}
	value, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, err
	}
	return value * 1024, nil
}
