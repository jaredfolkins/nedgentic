package frontend

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"nedgentic/internal/backend/runtime"
)

func TestParseDesktopLine(t *testing.T) {
	port, password, err := parseDesktopLine([]byte("noise\nNED_DESKTOP port=6080 password=Ab3dEf7h\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 6080 || password != "Ab3dEf7h" {
		t.Fatalf("got port=%d password=%q", port, password)
	}
}

func TestParseDesktopLineRejectsBadOutput(t *testing.T) {
	for _, out := range []string{
		"",
		"started\n",
		"NED_DESKTOP port=0 password=secret99\n",
		"NED_DESKTOP port=http password=secret99\n",
		"NED_DESKTOP port=6080\n",
	} {
		_, _, err := parseDesktopLine([]byte(out))
		if err == nil {
			t.Fatalf("expected error for %q", out)
		}
		if strings.Contains(err.Error(), "secret99") {
			t.Fatalf("error leaked the password: %v", err)
		}
	}
}

func TestDesktopURL(t *testing.T) {
	got := desktopURL("192.168.64.5", 6080, "Ab3dEf7h")
	want := "http://192.168.64.5:6080/vnc.html?autoconnect=1&password=Ab3dEf7h&resize=scale"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	display := desktopURL("192.168.64.5", 6080, "")
	if strings.Contains(display, "password") {
		t.Fatalf("display URL must omit the password: %s", display)
	}
	if got := desktopURL("192.168.64.5", 6080, "a&b=c"); !strings.Contains(got, "password=a%26b%3Dc") {
		t.Fatalf("expected an escaped password, got %s", got)
	}
}

func TestContainerIPv4(t *testing.T) {
	entry := runtime.ContainerListEntry{Networks: []runtime.ContainerListNetwork{
		{IPv4Address: ""},
		{IPv4Address: "192.168.64.137/24"},
	}}
	if got := containerIPv4(entry); got != "192.168.64.137" {
		t.Fatalf("got %q", got)
	}
	if got := containerIPv4(runtime.ContainerListEntry{}); got != "" {
		t.Fatalf("expected no IP, got %q", got)
	}
	bad := runtime.ContainerListEntry{Networks: []runtime.ContainerListNetwork{{IPv4Address: "not-an-ip"}}}
	if got := containerIPv4(bad); got != "" {
		t.Fatalf("expected invalid IP to be ignored, got %q", got)
	}
}

func TestContainerIPv4FromListJSON(t *testing.T) {
	// container 1.6.0 nests networks under status.
	var entries []runtime.ContainerListEntry
	raw := `[{"status":{"state":"running","networks":[{"ipv4Address":"192.168.64.9/24"}]},"configuration":{"id":"work"}}]`
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		t.Fatalf("parse: %v", err)
	}
	entry, ok := findContainerEntry(entries, "work")
	if !ok {
		t.Fatal("expected entry")
	}
	if got := containerIPv4(entry); got != "192.168.64.9" {
		t.Fatalf("got %q", got)
	}
	if !isRunningStatus(entry.Status) {
		t.Fatalf("expected running status, got %q", entry.Status)
	}
}

func TestDesktopVMName(t *testing.T) {
	got, err := desktopVMName(nil)
	if err != nil || got != "default" {
		t.Fatalf("expected default VM name, got %q (%v)", got, err)
	}
	got, err = desktopVMName([]string{"work-a"})
	if err != nil || got != "work-a" {
		t.Fatalf("expected work-a, got %q (%v)", got, err)
	}
	if _, err := desktopVMName([]string{"bad name;"}); err == nil {
		t.Fatal("expected invalid VM name to fail")
	}
}

func TestDesktopExecArgsRunAsHostUser(t *testing.T) {
	target := desktopTarget{
		Name:    "work",
		Workdir: "/home/alice/src/app",
		Env:     []string{"HOME=/home/alice", "SHELL=/bin/zsh"},
		UID:     501,
		GID:     20,
	}
	got := target.execArgs("start")
	want := []string{
		"exec", "--uid", "501", "--gid", "20",
		"--env", "HOME=/home/alice", "--env", "SHELL=/bin/zsh",
		"--workdir", "/home/alice/src/app",
		"work", desktopGuestCommand, "start",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}
