package frontend

import (
	"strings"

	sharedruntime "nedgentic/internal/runtime"
)

func normalizeAgentName(kind, raw string) (string, error) {
	return sharedruntime.NormalizeCLIIdentifier(kind, raw)
}

func normalizeAgentReference(kind, raw string) (string, error) {
	return sharedruntime.NormalizeCLIReference(kind, raw)
}

func normalizeAgentPath(kind, raw string) (string, error) {
	return sharedruntime.NormalizeCLIPath(kind, raw)
}

func sanitizeCommandOutputLines(out []byte) []string {
	return sharedruntime.SanitizeStructuredCommandLines(out)
}

func formatCommandOutputError(err error, out []byte) error {
	return sharedruntime.FormatCommandOutputError(err, out)
}

func normalizeRuntimeLineItems(kind string, out []byte) ([]string, error) {
	lines := sanitizeCommandOutputLines(out)
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		value, err := normalizeAgentName(kind, line)
		if err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, nil
}

func fieldOrDashText(value string) string {
	lines := sanitizeCommandOutputLines([]byte(value))
	if len(lines) == 0 {
		return "-"
	}
	return strings.Join(lines, " ")
}

func fieldOrDashIdentifier(kind, value string) string {
	normalized, err := normalizeAgentName(kind, value)
	if err != nil {
		return "-"
	}
	return normalized
}

func fieldOrDashReference(kind, value string) string {
	normalized, err := normalizeAgentReference(kind, value)
	if err != nil {
		return "-"
	}
	return normalized
}

func fieldOrDashPath(kind, value string) string {
	normalized, err := normalizeAgentPath(kind, value)
	if err != nil {
		return "-"
	}
	return normalized
}
