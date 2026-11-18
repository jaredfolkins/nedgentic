package frontend

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
	"nedgentic/internal/output"
)

var (
	confirmInput = os.Stdin
	confirmIsTTY = func() bool {
		return term.IsTerminal(int(os.Stdin.Fd()))
	}
)

// confirmMutation asks a y/N question on stdin. --yes skips the prompt; without
// a terminal it fails instead of blocking on stdin.
func confirmMutation(prompt string, yes bool) (bool, error) {
	if yes {
		return true, nil
	}
	if !confirmIsTTY() {
		return false, fmt.Errorf("confirmation required in non-interactive mode; rerun with --yes")
	}

	output.PrintInlinef("%s [y/N]: ", prompt)
	reader := bufio.NewReader(confirmInput)
	input, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(input), "y"), nil
}
