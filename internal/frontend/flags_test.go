package frontend

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	"nedgentic/internal/profile"
)

func TestApplyFlagOverridesInitFlags(t *testing.T) {
	flagInit = false
	flagInitImage = ""
	flagSystemd = false
	defer func() {
		flagInit = false
		flagInitImage = ""
		flagSystemd = false
	}()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().BoolVar(&flagInit, "init", false, "")
	cmd.Flags().StringVar(&flagInitImage, "init-image", "", "")
	cmd.Flags().BoolVar(&flagSystemd, "systemd", false, "")
	if err := cmd.Flags().Parse([]string{"--init", "--init-image", "docker.io/library/custom-init:latest", "--systemd"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
	if flags.Init == nil || !*flags.Init {
		t.Fatalf("expected init override to be true")
	}
	if flags.InitImage != "docker.io/library/custom-init:latest" {
		t.Fatalf("expected init-image override, got %q", flags.InitImage)
	}
	if flags.Systemd == nil || !*flags.Systemd {
		t.Fatalf("expected systemd override to be true")
	}
}

func TestApplyFlagOverridesNetworking(t *testing.T) {
	flagNetwork = ""
	flagNoDNS = false
	flagDNS = nil
	flagDNSDomain = ""
	flagDNSSearch = nil
	flagDNSOptions = nil
	defer func() {
		flagNetwork = ""
		flagNoDNS = false
		flagDNS = nil
		flagDNSDomain = ""
		flagDNSSearch = nil
		flagDNSOptions = nil
	}()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringVar(&flagNetwork, "network", "", "")
	cmd.Flags().BoolVar(&flagNoDNS, "no-dns", false, "")
	cmd.Flags().StringArrayVar(&flagDNS, "dns", nil, "")
	cmd.Flags().StringVar(&flagDNSDomain, "dns-domain", "", "")
	cmd.Flags().StringArrayVar(&flagDNSSearch, "dns-search", nil, "")
	cmd.Flags().StringArrayVar(&flagDNSOptions, "dns-option", nil, "")
	if err := cmd.Flags().Parse([]string{
		"--network", "ned-lab",
		"--no-dns",
		"--dns", "203.0.113.53",
		"--dns=198.51.100.2",
		"--dns-domain", "example.test",
		"--dns-search", "svc.local",
		"--dns-search", "corp.local",
		"--dns-option", "ndots:1",
		"--dns-option", "timeout:2",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
	if flags.Network != "ned-lab" {
		t.Fatalf("expected network override, got %q", flags.Network)
	}
	if flags.NoDNS == nil || !*flags.NoDNS {
		t.Fatalf("expected no-dns override to be true")
	}
	wantDNS := []string{"203.0.113.53", "198.51.100.2"}
	if len(flags.DNS) != len(wantDNS) {
		t.Fatalf("unexpected dns count: got %d want %d (%v)", len(flags.DNS), len(wantDNS), flags.DNS)
	}
	for i := range wantDNS {
		if flags.DNS[i] != wantDNS[i] {
			t.Fatalf("dns %d mismatch: got %q want %q", i, flags.DNS[i], wantDNS[i])
		}
	}
	if flags.DNSDomain != "example.test" {
		t.Fatalf("expected dns-domain override, got %q", flags.DNSDomain)
	}
	wantSearch := []string{"svc.local", "corp.local"}
	if len(flags.DNSSearch) != len(wantSearch) {
		t.Fatalf("unexpected dns-search count: got %d want %d (%v)", len(flags.DNSSearch), len(wantSearch), flags.DNSSearch)
	}
	for i := range wantSearch {
		if flags.DNSSearch[i] != wantSearch[i] {
			t.Fatalf("dns-search %d mismatch: got %q want %q", i, flags.DNSSearch[i], wantSearch[i])
		}
	}
	wantOptions := []string{"ndots:1", "timeout:2"}
	if len(flags.DNSOptions) != len(wantOptions) {
		t.Fatalf("unexpected dns-option count: got %d want %d (%v)", len(flags.DNSOptions), len(wantOptions), flags.DNSOptions)
	}
	for i := range wantOptions {
		if flags.DNSOptions[i] != wantOptions[i] {
			t.Fatalf("dns-option %d mismatch: got %q want %q", i, flags.DNSOptions[i], wantOptions[i])
		}
	}
}

func TestApplyFlagOverridesProxy(t *testing.T) {
	flagProxy = ""
	flagHTTPProxy = ""
	flagHTTPSProxy = ""
	flagNoProxy = nil
	defer func() {
		flagProxy = ""
		flagHTTPProxy = ""
		flagHTTPSProxy = ""
		flagNoProxy = nil
	}()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringVar(&flagProxy, "proxy", "", "")
	cmd.Flags().StringVar(&flagHTTPProxy, "http-proxy", "", "")
	cmd.Flags().StringVar(&flagHTTPSProxy, "https-proxy", "", "")
	cmd.Flags().StringArrayVar(&flagNoProxy, "no-proxy", nil, "")
	if err := cmd.Flags().Parse([]string{
		"--proxy", "http://proxy.base:8080",
		"--http-proxy", "http://proxy.http:8080",
		"--https-proxy", "http://proxy.https:8443",
		"--no-proxy", "localhost",
		"--no-proxy", "127.0.0.1,.svc.cluster.local",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
	if flags.Proxy != "http://proxy.base:8080" {
		t.Fatalf("unexpected proxy: %q", flags.Proxy)
	}
	if flags.HTTPProxy != "http://proxy.http:8080" {
		t.Fatalf("unexpected http-proxy: %q", flags.HTTPProxy)
	}
	if flags.HTTPSProxy != "http://proxy.https:8443" {
		t.Fatalf("unexpected https-proxy: %q", flags.HTTPSProxy)
	}
	wantNoProxy := []string{"localhost", "127.0.0.1,.svc.cluster.local"}
	if len(flags.NoProxy) != len(wantNoProxy) {
		t.Fatalf("unexpected no-proxy count: got %d want %d (%v)", len(flags.NoProxy), len(wantNoProxy), flags.NoProxy)
	}
	for i := range wantNoProxy {
		if flags.NoProxy[i] != wantNoProxy[i] {
			t.Fatalf("no-proxy %d mismatch: got %q want %q", i, flags.NoProxy[i], wantNoProxy[i])
		}
	}
}

func TestApplyFlagOverridesGentic(t *testing.T) {
	flagGenticDir = ""
	flagGenticDebug = false
	defer func() {
		flagGenticDir = ""
		flagGenticDebug = false
	}()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringVar(&flagGenticDir, "gentic-dir", "", "")
	cmd.Flags().BoolVar(&flagGenticDebug, "gentic-debug", false, "")
	if err := cmd.Flags().Parse([]string{"--gentic-dir", "/opt/ned/hooks", "--gentic-debug"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
	if flags.GenticDir != "/opt/ned/hooks" {
		t.Fatalf("expected gentic-dir override to be set")
	}
	if flags.GenticDebug == nil || !*flags.GenticDebug {
		t.Fatalf("expected gentic-debug override to be true")
	}
}

func TestApplyFlagOverridesUlimitMergesProfileDefaults(t *testing.T) {
	flagUlimits = nil
	defer func() {
		flagUlimits = nil
	}()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringArrayVar(&flagUlimits, "ulimit", nil, "")
	if err := cmd.Flags().Parse([]string{"--ulimit", "nofile=2048:4096", "--ulimit", "core=0"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	base := profile.DefaultFlags()
	base.Ulimits = []string{"nofile=1024:2048", "nproc=512"}
	flags := applyFlagOverrides(base, cmd)
	want := []string{"nofile=2048:4096", "nproc=512", "core=0"}
	if len(flags.Ulimits) != len(want) {
		t.Fatalf("unexpected ulimit count: got %d want %d (%v)", len(flags.Ulimits), len(want), flags.Ulimits)
	}
	for i := range want {
		if flags.Ulimits[i] != want[i] {
			t.Fatalf("ulimit %d mismatch: got %q want %q (%v)", i, flags.Ulimits[i], want[i], flags.Ulimits)
		}
	}
}

func TestApplyFlagOverridesCapabilities(t *testing.T) {
	flagCapAdd = nil
	flagCapDrop = nil
	defer func() {
		flagCapAdd = nil
		flagCapDrop = nil
	}()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringArrayVar(&flagCapAdd, "cap-add", nil, "")
	cmd.Flags().StringArrayVar(&flagCapDrop, "cap-drop", nil, "")
	if err := cmd.Flags().Parse([]string{"--cap-add", "NET_ADMIN", "--cap-add", "SYS_PTRACE", "--cap-drop", "MKNOD"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
	if got, want := flags.CapAdd, []string{"NET_ADMIN", "SYS_PTRACE"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected cap-add values: got %#v want %#v", got, want)
	}
	if got, want := flags.CapDrop, []string{"MKNOD"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected cap-drop values: got %#v want %#v", got, want)
	}
}
