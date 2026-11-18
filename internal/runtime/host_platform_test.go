package runtime

import (
	"strings"
	"testing"
)

func TestCheckDarwinHost(t *testing.T) {
	for _, tc := range []struct {
		name    string
		goarch  string
		swVers  string
		wantErr string
	}{
		{name: "tahoe arm64", goarch: "arm64", swVers: "26.5.2\n"},
		{name: "future major", goarch: "arm64", swVers: "27.0"},
		{name: "intel build", goarch: "amd64", swVers: "26.1", wantErr: "Apple silicon"},
		{name: "sequoia", goarch: "arm64", swVers: "15.7.1", wantErr: "requires macOS 26 or later for Apple's container runtime; detected macOS 15.7.1"},
		{name: "unparseable", goarch: "arm64", swVers: "garbage", wantErr: "unable to parse macOS version"},
		{name: "empty", goarch: "arm64", swVers: "", wantErr: "unable to parse macOS version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkDarwinHost(tc.goarch, tc.swVers)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("checkDarwinHost(%q, %q) error: %v", tc.goarch, tc.swVers, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("checkDarwinHost(%q, %q) = %v, want error containing %q", tc.goarch, tc.swVers, err, tc.wantErr)
			}
		})
	}
}

func TestParseMacOSVersion(t *testing.T) {
	version, ok := parseMacOSVersion("26.2")
	if !ok {
		t.Fatalf("expected version parse to succeed")
	}
	if version.Major != 26 || version.Minor != 2 || version.Patch != 0 {
		t.Fatalf("unexpected version parse: %+v", version)
	}
}
