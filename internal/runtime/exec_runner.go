package runtime

import (
	"bytes"
	"os"
	"os/exec"
)

func RunContainerExec(bin string, args []string) error {
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// ContainerExecOutput runs a container exec and returns its stdout. Stdout
// may carry secrets, so only stderr is folded into a returned error.
func ContainerExecOutput(bin string, args []string) ([]byte, error) {
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, FormatCommandOutputError(err, stderr.Bytes())
	}
	return stdout.Bytes(), nil
}
