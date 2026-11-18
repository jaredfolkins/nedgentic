package frontend

import "nedgentic/internal/output"

func printRuntimeResolveError(err error) {
	output.Printf("Error resolving container runtime: %v", err)
}
