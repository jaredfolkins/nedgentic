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
