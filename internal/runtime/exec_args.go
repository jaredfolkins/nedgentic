package runtime

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

type ExecOptions struct {
	TTY           bool
	UID           *int
	GID           *int
	Env           []string
	Ulimits       []string
	Workdir       string
	ContainerName string
	Command       []string
}

func WantsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func BuildExecArgs(opts ExecOptions) ([]string, error) {
	name := strings.TrimSpace(opts.ContainerName)
	if name == "" {
		return nil, fmt.Errorf("container name is required")
	}
	if len(opts.Command) == 0 {
		return nil, fmt.Errorf("exec command is required")
	}

	args := []string{"exec"}
	if opts.TTY {
		args = append(args, "-it")
	}
	if opts.UID != nil {
		args = append(args, "--uid", strconv.Itoa(*opts.UID))
	}
	if opts.GID != nil {
		args = append(args, "--gid", strconv.Itoa(*opts.GID))
	}
	for _, env := range opts.Env {
		if env == "" {
			continue
		}
		args = append(args, "--env", env)
	}
	ulimits, err := NormalizeUlimits(opts.Ulimits)
	if err != nil {
		return nil, err
	}
	for _, limit := range ulimits {
		args = append(args, "--ulimit", limit)
	}
	if workdir := strings.TrimSpace(opts.Workdir); workdir != "" {
		args = append(args, "--workdir", workdir)
	}
	args = append(args, name)
	args = append(args, opts.Command...)
	return args, nil
}
