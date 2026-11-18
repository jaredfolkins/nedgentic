package runtime

import (
	"strings"

	"nedgentic/internal/profile"
)

type ProxySettings struct {
	HTTP    string   `json:"http,omitempty"`
	HTTPS   string   `json:"https,omitempty"`
	NoProxy []string `json:"no_proxy,omitempty"`
}

func ProxySettingsFromFlags(flags profile.Flags) ProxySettings {
	base := strings.TrimSpace(flags.Proxy)
	httpProxy := strings.TrimSpace(flags.HTTPProxy)
	httpsProxy := strings.TrimSpace(flags.HTTPSProxy)
	if httpProxy == "" && base != "" {
		httpProxy = base
	}
	if httpsProxy == "" && base != "" {
		httpsProxy = base
	}
	return ProxySettings{
		HTTP:    httpProxy,
		HTTPS:   httpsProxy,
		NoProxy: normalizeNoProxyEntries(flags.NoProxy),
	}
}

func ProxyEnvEntries(flags profile.Flags) []string {
	return ProxyEnvEntriesFromSettings(ProxySettingsFromFlags(flags))
}

func ProxyEnvEntriesFromSettings(settings ProxySettings) []string {
	noProxy := strings.Join(normalizeNoProxyEntries(settings.NoProxy), ",")
	httpProxy := strings.TrimSpace(settings.HTTP)
	httpsProxy := strings.TrimSpace(settings.HTTPS)
	if httpProxy == "" && httpsProxy == "" && noProxy == "" {
		return nil
	}
	env := make([]string, 0, 6)
	if httpProxy != "" {
		env = append(env, "HTTP_PROXY="+httpProxy, "http_proxy="+httpProxy)
	}
	if httpsProxy != "" {
		env = append(env, "HTTPS_PROXY="+httpsProxy, "https_proxy="+httpsProxy)
	}
	if noProxy != "" {
		env = append(env, "NO_PROXY="+noProxy, "no_proxy="+noProxy)
	}
	return env
}

func (settings ProxySettings) IsZero() bool {
	return strings.TrimSpace(settings.HTTP) == "" &&
		strings.TrimSpace(settings.HTTPS) == "" &&
		len(normalizeNoProxyEntries(settings.NoProxy)) == 0
}

func normalizeNoProxyEntries(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, raw := range values {
		for _, value := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}
