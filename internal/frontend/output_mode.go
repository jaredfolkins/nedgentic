package frontend

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"nedgentic/internal/output"
)

// outputFlag is the --output flag on list and status commands.
const outputFlag = "output"

type listResponse[T any] struct {
	Kind    string `json:"kind"`
	Items   []T    `json:"items"`
	Summary any    `json:"summary,omitempty"`
}

type singleResponse[T any] struct {
	Kind string `json:"kind"`
	Item T      `json:"item"`
}

// tableField is one column of a text table.
type tableField[T any] struct {
	Header string
	Value  func(T) string
}

// addOutputFlag registers --output text|json on a read command.
func addOutputFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&flagOutput, outputFlag, "", "Output format (text or json)")
}

func configureOutput() error {
	mode, err := output.ParseMode(strings.TrimSpace(flagOutput))
	if err != nil {
		return err
	}
	output.Configure(output.Config{
		Mode:        mode,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		StdoutIsTTY: writerIsTTY(os.Stdout),
		StderrIsTTY: writerIsTTY(os.Stderr),
	})
	return nil
}

func currentOutputMode() output.Mode {
	return output.CurrentMode()
}

// renderOutput writes value as indented JSON with --output json and calls text
// otherwise.
func renderOutput(cmd *cobra.Command, value any, text func(io.Writer) error) error {
	writer := commandStdout(cmd)
	if currentOutputMode() == output.ModeJSON {
		return writeJSON(writer, value)
	}
	return text(writer)
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeLines(writer io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
	}
	return nil
}

func writeTable(writer io.Writer, header []string, rows [][]string) error {
	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	if len(header) > 0 {
		if _, err := fmt.Fprintln(tw, strings.Join(header, "\t")); err != nil {
			return err
		}
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	return writeLines(writer, splitNonEmptyLines(buf.String()))
}

// writeItemTable writes items as a table, or emptyMessage when there are none.
func writeItemTable[T any](writer io.Writer, items []T, fields []tableField[T], emptyMessage string) error {
	if len(items) == 0 {
		return writeLines(writer, []string{emptyMessage})
	}
	headers := make([]string, 0, len(fields))
	for _, field := range fields {
		headers = append(headers, field.Header)
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		row := make([]string, 0, len(fields))
		for _, field := range fields {
			row = append(row, field.Value(item))
		}
		rows = append(rows, row)
	}
	return writeTable(writer, headers, rows)
}

func writeCommandOutputText(writer io.Writer, out []byte) error {
	return writeLines(writer, splitNonEmptyLines(string(out)))
}

func splitNonEmptyLines(value string) []string {
	trimmed := strings.TrimRight(value, "\n")
	if strings.TrimSpace(trimmed) == "" {
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		filtered = append(filtered, line)
	}
	return filtered
}

func commandStdout(cmd *cobra.Command) io.Writer {
	if cmd != nil {
		return cmd.OutOrStdout()
	}
	return output.StdoutWriter()
}

func writerIsTTY(writer io.Writer) bool {
	type fdWriter interface {
		Fd() uintptr
	}
	fileWriter, ok := writer.(fdWriter)
	if !ok {
		return false
	}
	return term.IsTerminal(int(fileWriter.Fd()))
}
