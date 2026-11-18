package frontend

import (
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
)

func runWithSpinner(label string, fn func() error) error {
	return runWithSpinnerMode(label, true, fn)
}

func runWithSpinnerStep(label string, fn func() error) error {
	return runWithSpinnerMode(label, false, fn)
}

func runWithSpinnerMode(label string, emitFinalLine bool, fn func() error) error {
	return output.RunWithSpinner(label, output.SpinnerOptions{
		Interactive:   runtime.WantsTTY(),
		EmitFinalLine: emitFinalLine,
	}, fn)
}
