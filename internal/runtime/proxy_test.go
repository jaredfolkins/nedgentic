package runtime

import (
	"reflect"
	"testing"

	"nedgentic/internal/profile"
)

func TestProxySettingsFromFlagsUsesBaseProxyFallback(t *testing.T) {
	flags := profile.DefaultFlags()
	flags.Proxy = "http://proxy.base:8080"
	flags.NoProxy = []string{"localhost", "127.0.0.1,.svc.cluster.local", "LOCALHOST"}

	got := ProxySettingsFromFlags(flags)
	want := ProxySettings{
		HTTP:    "http://proxy.base:8080",
		HTTPS:   "http://proxy.base:8080",
		NoProxy: []string{"localhost", "127.0.0.1", ".svc.cluster.local"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected proxy settings:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestProxySettingsFromFlagsExplicitOverridesBase(t *testing.T) {
	flags := profile.DefaultFlags()
	flags.Proxy = "http://proxy.base:8080"
	flags.HTTPProxy = "http://proxy.http:8081"
	flags.HTTPSProxy = "http://proxy.https:8443"

	got := ProxySettingsFromFlags(flags)
	if got.HTTP != "http://proxy.http:8081" {
		t.Fatalf("unexpected http proxy: %q", got.HTTP)
	}
	if got.HTTPS != "http://proxy.https:8443" {
		t.Fatalf("unexpected https proxy: %q", got.HTTPS)
	}
}

func TestProxyEnvEntriesFromSettings(t *testing.T) {
	settings := ProxySettings{
		HTTP:    "http://proxy.http:8080",
		HTTPS:   "http://proxy.https:8443",
		NoProxy: []string{"localhost", "127.0.0.1"},
	}
	got := ProxyEnvEntriesFromSettings(settings)
	want := []string{
		"HTTP_PROXY=http://proxy.http:8080",
		"http_proxy=http://proxy.http:8080",
		"HTTPS_PROXY=http://proxy.https:8443",
		"https_proxy=http://proxy.https:8443",
		"NO_PROXY=localhost,127.0.0.1",
		"no_proxy=localhost,127.0.0.1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected proxy env:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestProxyEnvEntriesEmpty(t *testing.T) {
	if got := ProxyEnvEntries(profile.DefaultFlags()); len(got) != 0 {
		t.Fatalf("expected no proxy env, got %#v", got)
	}
	if !(ProxySettings{}).IsZero() {
		t.Fatalf("expected zero proxy settings")
	}
}
