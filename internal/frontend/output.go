package frontend

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"golang.org/x/term"
	"nedgentic/internal/output"
)

// tableField is one column of a text table.
type tableField[T any] struct {
	Header string
	Value  func(T) string
}

func configureOutput() {
	output.Configure(output.Config{
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		StdoutIsTTY: writerIsTTY(os.Stdout),
		StderrIsTTY: writerIsTTY(os.Stderr),
	})
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
