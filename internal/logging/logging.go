package logging

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"nedgentic/internal/output"
)

type Level int32

const (
	Debug Level = iota
	Info
	Warn
	Error
)

var currentLevel int32 = int32(Info)

func ParseLevel(value string) Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return Debug
	case "info":
		return Info
	case "warn", "warning":
		return Warn
	case "error":
		return Error
	default:
		return Info
	}
}

func SetLevel(level Level) {
	atomic.StoreInt32(&currentLevel, int32(level))
}

func SetLevelFromEnv(value string) {
	SetLevel(ParseLevel(value))
}

func enabled(level Level) bool {
	return level >= Level(atomic.LoadInt32(&currentLevel))
}

func Logf(level Level, format string, args ...any) {
	if !enabled(level) {
		return
	}
	message := fmt.Sprintf("%s: %s", levelString(level), fmt.Sprintf(format, args...))
	fmt.Fprintln(os.Stdout, output.Format(message))
}

func Debugf(format string, args ...any) { Logf(Debug, format, args...) }
func Infof(format string, args ...any)  { Logf(Info, format, args...) }
func Warnf(format string, args ...any)  { Logf(Warn, format, args...) }
func Errorf(format string, args ...any) { Logf(Error, format, args...) }

func Fatalf(format string, args ...any) {
	Logf(Error, format, args...)
	os.Exit(1)
}

func levelString(level Level) string {
	switch level {
	case Debug:
		return "Debug"
	case Info:
		return "Info"
	case Warn:
		return "Warn"
	case Error:
		return "Error"
	default:
		return "Info"
	}
}
