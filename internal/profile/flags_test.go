package profile

import (
	"reflect"
	"testing"
)

func TestMergeUlimitsPreservesOrderAndOverridesByType(t *testing.T) {
	base := []string{"nofile=1024:1024", "nproc=512"}
	override := []string{"nofile=2048:2048", "core=0"}
	got := MergeUlimits(base, override)
	want := []string{"nofile=2048:2048", "nproc=512", "core=0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected merged ulimits:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestMergeFlagsMergesUlimits(t *testing.T) {
	base := DefaultFlags()
	base.Ulimits = []string{"nofile=1024:1024", "nproc=512"}
	override := Flags{
		Ulimits: []string{"nproc=2048", "core=0"},
	}
	merged := MergeFlags(base, override)
	want := []string{"nofile=1024:1024", "nproc=2048", "core=0"}
	if !reflect.DeepEqual(merged.Ulimits, want) {
		t.Fatalf("unexpected merged flags ulimits:\nwant: %#v\ngot:  %#v", want, merged.Ulimits)
	}
}

func TestMergeFlagsOverridesCapabilities(t *testing.T) {
	base := DefaultFlags()
	base.CapAdd = []string{"NET_RAW"}
	base.CapDrop = []string{"MKNOD"}
	override := Flags{
		CapAdd:  []string{"NET_ADMIN"},
		CapDrop: []string{"ALL"},
	}
	merged := MergeFlags(base, override)
	if !reflect.DeepEqual(merged.CapAdd, []string{"NET_ADMIN"}) {
		t.Fatalf("unexpected cap-add values: %#v", merged.CapAdd)
	}
	if !reflect.DeepEqual(merged.CapDrop, []string{"ALL"}) {
		t.Fatalf("unexpected cap-drop values: %#v", merged.CapDrop)
	}
}

func TestMergeFlagsMergesNetworkingFields(t *testing.T) {
	base := DefaultFlags()
	base.Network = "default-net"
	base.DNS = []string{"203.0.113.53"}
	base.DNSSearch = []string{"base.local"}
	base.DNSOptions = []string{"ndots:1"}

	noDNS := true
	override := Flags{
		Network:    "custom-net",
		NoDNS:      &noDNS,
		DNS:        []string{"198.51.100.2"},
		DNSDomain:  "example.test",
		DNSSearch:  []string{"svc.local"},
		DNSOptions: []string{"timeout:2"},
	}

	merged := MergeFlags(base, override)
	if merged.Network != "custom-net" {
		t.Fatalf("unexpected network: %q", merged.Network)
	}
	if merged.NoDNS == nil || !*merged.NoDNS {
		t.Fatalf("expected no-dns to be true")
	}
	if !reflect.DeepEqual(merged.DNS, []string{"198.51.100.2"}) {
		t.Fatalf("unexpected dns values: %#v", merged.DNS)
	}
	if merged.DNSDomain != "example.test" {
		t.Fatalf("unexpected dns-domain: %q", merged.DNSDomain)
	}
	if !reflect.DeepEqual(merged.DNSSearch, []string{"svc.local"}) {
		t.Fatalf("unexpected dns-search values: %#v", merged.DNSSearch)
	}
	if !reflect.DeepEqual(merged.DNSOptions, []string{"timeout:2"}) {
		t.Fatalf("unexpected dns-option values: %#v", merged.DNSOptions)
	}
}

func TestMergeFlagsMergesProxyFields(t *testing.T) {
	base := DefaultFlags()
	base.Proxy = "http://proxy.default:8080"
	base.HTTPProxy = "http://proxy.http:8080"
	base.HTTPSProxy = "http://proxy.https:8443"
	base.NoProxy = []string{"localhost"}

	override := Flags{
		Proxy:      "http://proxy.override:8080",
		HTTPProxy:  "http://proxy.http.override:8080",
		HTTPSProxy: "http://proxy.https.override:8443",
		NoProxy:    []string{"127.0.0.1", ".svc.cluster.local"},
	}

	merged := MergeFlags(base, override)
	if merged.Proxy != "http://proxy.override:8080" {
		t.Fatalf("unexpected proxy: %q", merged.Proxy)
	}
	if merged.HTTPProxy != "http://proxy.http.override:8080" {
		t.Fatalf("unexpected http-proxy: %q", merged.HTTPProxy)
	}
	if merged.HTTPSProxy != "http://proxy.https.override:8443" {
		t.Fatalf("unexpected https-proxy: %q", merged.HTTPSProxy)
	}
	if !reflect.DeepEqual(merged.NoProxy, []string{"127.0.0.1", ".svc.cluster.local"}) {
		t.Fatalf("unexpected no-proxy values: %#v", merged.NoProxy)
	}
}

func TestMergeFlagsMergesInitAndSystemd(t *testing.T) {
	base := DefaultFlags()
	baseInit := true
	baseSystemd := false
	base.Init = &baseInit
	base.InitImage = "docker.io/library/base-init:latest"
	base.Systemd = &baseSystemd

	overrideInit := false
	overrideSystemd := true
	override := Flags{
		Init:      &overrideInit,
		InitImage: "docker.io/library/override-init:latest",
		Systemd:   &overrideSystemd,
	}

	merged := MergeFlags(base, override)
	if merged.Init == nil || *merged.Init {
		t.Fatalf("expected init to be false after override")
	}
	if merged.InitImage != "docker.io/library/override-init:latest" {
		t.Fatalf("unexpected init-image: %q", merged.InitImage)
	}
	if merged.Systemd == nil || !*merged.Systemd {
		t.Fatalf("expected systemd to be true after override")
	}
}
