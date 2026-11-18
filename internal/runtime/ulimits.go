package runtime

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const ulimitFormatHint = "<type>=<soft>[:<hard>]"

var ulimitTypePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func normalizeSingleUlimit(raw string) (normalized string, key string, err error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "", nil
	}
	if strings.ContainsAny(value, " ,") {
		return "", "", fmt.Errorf("invalid ulimit %q: values must not contain spaces or commas (expected %s)", value, ulimitFormatHint)
	}
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid ulimit %q: expected %s", value, ulimitFormatHint)
	}
	limitType := strings.TrimSpace(parts[0])
	if limitType == "" || !ulimitTypePattern.MatchString(limitType) {
		return "", "", fmt.Errorf("invalid ulimit %q: invalid resource type %q", value, limitType)
	}
	limits := strings.Split(parts[1], ":")
	if len(limits) > 2 {
		return "", "", fmt.Errorf("invalid ulimit %q: expected %s", value, ulimitFormatHint)
	}
	softRaw := strings.TrimSpace(limits[0])
	if softRaw == "" {
		return "", "", fmt.Errorf("invalid ulimit %q: soft limit is required", value)
	}
	soft, err := strconv.ParseUint(softRaw, 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("invalid ulimit %q: soft limit must be an unsigned integer", value)
	}
	if len(limits) == 1 {
		return fmt.Sprintf("%s=%d", limitType, soft), strings.ToLower(limitType), nil
	}
	hardRaw := strings.TrimSpace(limits[1])
	if hardRaw == "" {
		return "", "", fmt.Errorf("invalid ulimit %q: hard limit is required when ':' is present", value)
	}
	hard, err := strconv.ParseUint(hardRaw, 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("invalid ulimit %q: hard limit must be an unsigned integer", value)
	}
	if hard < soft {
		return "", "", fmt.Errorf("invalid ulimit %q: hard limit must be greater than or equal to soft limit", value)
	}
	return fmt.Sprintf("%s=%d:%d", limitType, soft, hard), strings.ToLower(limitType), nil
}

// NormalizeUlimits validates and canonicalizes ulimit entries while ensuring a
// deterministic override order (last value wins per resource type).
func NormalizeUlimits(values []string) ([]string, error) {
	normalized := make([]string, 0, len(values))
	indexByKey := map[string]int{}
	for _, raw := range values {
		value, key, err := normalizeSingleUlimit(raw)
		if err != nil {
			return nil, err
		}
		if value == "" {
			continue
		}
		if idx, ok := indexByKey[key]; ok {
			normalized[idx] = value
			continue
		}
		indexByKey[key] = len(normalized)
		normalized = append(normalized, value)
	}
	return normalized, nil
}
