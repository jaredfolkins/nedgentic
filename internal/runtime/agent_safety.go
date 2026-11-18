package runtime

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/google/go-containerregistry/pkg/name"
)

type commandValueOptions struct {
	allowWhitespace   bool
	allowPathChars    bool
	rejectQuerySuffix bool
	rejectURL         bool
}

func NormalizeCLIIdentifier(kind, raw string) (string, error) {
	return normalizeCommandValue(kind, raw, commandValueOptions{
		allowWhitespace:   false,
		allowPathChars:    false,
		rejectQuerySuffix: true,
	})
}

func NormalizeCLIReference(kind, raw string) (string, error) {
	value, err := normalizeCommandValue(kind, raw, commandValueOptions{
		allowWhitespace:   false,
		allowPathChars:    true,
		rejectQuerySuffix: true,
	})
	if err != nil {
		return "", err
	}
	if _, err := name.ParseReference(value, name.WithDefaultRegistry("docker.io"), name.WithDefaultTag("latest")); err != nil {
		return "", fmt.Errorf("invalid %s %q: %v", displayKind(kind), value, err)
	}
	return value, nil
}

func NormalizeCLIPath(kind, raw string) (string, error) {
	return normalizeCommandValue(kind, raw, commandValueOptions{
		allowWhitespace:   true,
		allowPathChars:    true,
		rejectQuerySuffix: true,
		rejectURL:         true,
	})
}

func SanitizeStructuredCommandOutput(output []byte) []byte {
	if len(output) == 0 {
		return nil
	}
	cleaned := stripANSISequences(string(output))
	cleaned = strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			return r
		case r == '\ufeff':
			return -1
		case unicode.IsControl(r):
			return -1
		default:
			return r
		}
	}, cleaned)
	return []byte(cleaned)
}

func SanitizeStructuredCommandText(output []byte) string {
	return strings.TrimSpace(string(SanitizeStructuredCommandOutput(output)))
}

func SanitizeStructuredCommandLines(output []byte) []string {
	cleaned := string(SanitizeStructuredCommandOutput(output))
	if strings.TrimSpace(cleaned) == "" {
		return nil
	}
	lines := strings.Split(cleaned, "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	return items
}

func FormatCommandOutputError(err error, output []byte) error {
	if err == nil {
		return nil
	}
	text := SanitizeStructuredCommandText(output)
	if text == "" {
		return err
	}
	return fmt.Errorf("%v: %s", err, text)
}

func normalizeCommandValue(kind, raw string, opts commandValueOptions) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("%s is required", displayKind(kind))
	}
	if hasShellQuoteWrapper(value) {
		return "", fmt.Errorf("%s must be passed without shell quotes", displayKind(kind))
	}
	if containsControlCharacter(value) {
		return "", fmt.Errorf("%s contains control characters", displayKind(kind))
	}
	if opts.rejectQuerySuffix && strings.ContainsAny(value, "?#") {
		return "", fmt.Errorf("%s must not contain query or fragment suffixes", displayKind(kind))
	}
	if opts.rejectURL && strings.Contains(value, "://") {
		return "", fmt.Errorf("%s must be a filesystem path, not a URL", displayKind(kind))
	}
	if !opts.allowWhitespace && containsWhitespace(value) {
		return "", fmt.Errorf("%s must not contain whitespace", displayKind(kind))
	}
	if !opts.allowPathChars {
		switch {
		case value == ".", value == "..":
			return "", fmt.Errorf("%s contains invalid path characters", displayKind(kind))
		case strings.ContainsAny(value, `/\`):
			return "", fmt.Errorf("%s contains invalid path characters", displayKind(kind))
		}
	}
	return value, nil
}

func displayKind(kind string) string {
	trimmed := strings.TrimSpace(kind)
	if trimmed == "" {
		return "value"
	}
	return trimmed
}

func hasShellQuoteWrapper(value string) bool {
	if len(value) < 2 {
		return false
	}
	first := value[0]
	last := value[len(value)-1]
	switch first {
	case '\'', '"', '`':
		return last == first
	default:
		return false
	}
}

func containsControlCharacter(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}

func containsWhitespace(value string) bool {
	return strings.IndexFunc(value, unicode.IsSpace) >= 0
}

func stripANSISequences(value string) string {
	if value == "" {
		return value
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for i := 0; i < len(value); {
		if value[i] != 0x1b {
			builder.WriteByte(value[i])
			i++
			continue
		}
		if i+1 >= len(value) {
			i++
			continue
		}
		switch value[i+1] {
		case '[':
			i += 2
			for i < len(value) {
				ch := value[i]
				i++
				if ch >= 0x40 && ch <= 0x7e {
					break
				}
			}
		case ']':
			i += 2
			for i < len(value) {
				if value[i] == '\a' {
					i++
					break
				}
				if value[i] == 0x1b && i+1 < len(value) && value[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		default:
			i += 2
		}
	}
	return builder.String()
}
