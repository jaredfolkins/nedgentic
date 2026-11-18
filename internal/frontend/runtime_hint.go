package frontend

import (
	"strings"

	"nedgentic/internal/output"
)

const runtimeInstallHint = "Run: ned runtime install"

func printRuntimeResolveError(err error) {
	output.Printf("Error resolving container runtime: %v", err)
	if shouldPrintRuntimeInstallHint(err) {
		output.Print(runtimeInstallHint)
	}
}

func shouldPrintRuntimeInstallHint(err error) bool {
	if err == nil {
		return false
	}
	return !strings.Contains(err.Error(), "ned runtime install")
}
