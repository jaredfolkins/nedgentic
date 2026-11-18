package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

const (
	prefixLeft   = "\u2310"
	prefixDot    = "\u25ef"
	prefixDash   = "-"
	prefixWhite  = "\x1b[38;5;15m"
	prefixSilver = "\x1b[38;5;251m"
	prefixHair   = "\x1b[38;5;180m"
	resetColor   = "\x1b[0m"
)

type Config struct {
	Stdout      io.Writer
	Stderr      io.Writer
	StdoutIsTTY bool
	StderrIsTTY bool
}

var (
	configMu sync.RWMutex
	config   = defaultConfig()
)

func defaultConfig() Config {
	return Config{
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		StdoutIsTTY: writerIsTTY(os.Stdout),
		StderrIsTTY: writerIsTTY(os.Stderr),
	}
}

func Configure(cfg Config) {
	configMu.Lock()
	defer configMu.Unlock()

	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	config = cfg
}

func Snapshot() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return config
}

func StdoutWriter() io.Writer {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.Stdout
}

func StderrWriter() io.Writer {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.Stderr
}

func Format(message string) string {
	if !shouldDecorate() {
		return message
	}
	return prefixHair + "[" + prefixWhite + prefixLeft +
		prefixSilver + prefixDot +
		prefixWhite + prefixDash +
		prefixSilver + prefixDot +
		prefixHair + "]" + resetColor + " " + message
}

func Print(message string) {
	_, _ = fmt.Fprintln(StderrWriter(), Format(message))
}

func Printf(format string, args ...any) {
	Print(fmt.Sprintf(format, args...))
}

func WriteFormattedLines(writer io.Writer, text string) {
	if writer == nil {
		return
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		_, _ = fmt.Fprintln(writer, Format(line))
	}
}

func shouldDecorate() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.StdoutIsTTY && config.StderrIsTTY
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
