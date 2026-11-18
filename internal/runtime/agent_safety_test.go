package runtime

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeCLIIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "valid", input: "sandbox-vm", want: "sandbox-vm"},
		{name: "quoted", input: "\"sandbox-vm\"", wantErr: "shell quotes"},
		{name: "control", input: "sandbox\nvm", wantErr: "control characters"},
		{name: "query", input: "sandbox-vm?debug=1", wantErr: "query or fragment"},
		{name: "path", input: "../sandbox", wantErr: "invalid path characters"},
		{name: "whitespace", input: "sandbox vm", wantErr: "whitespace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeCLIIdentifier("vm name", tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected value: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeCLIReference(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "valid", input: "docker.io/library/alpine:latest", want: "docker.io/library/alpine:latest"},
		{name: "quoted", input: "'alpine:latest'", wantErr: "shell quotes"},
		{name: "query", input: "docker.io/library/alpine:latest?download=1", wantErr: "query or fragment"},
		{name: "bad syntax", input: "docker.io/library/app@@", wantErr: "invalid image reference"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeCLIReference("image reference", tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected value: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeCLIPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "valid", input: "/tmp/out file.webm", want: "/tmp/out file.webm"},
		{name: "url", input: "https://example.com/out.webm", wantErr: "filesystem path"},
		{name: "fragment", input: "/tmp/out.webm#frag", wantErr: "query or fragment"},
		{name: "control", input: "/tmp/out\x00.webm", wantErr: "control characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeCLIPath("output path", tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected value: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeStructuredCommandLines(t *testing.T) {
	got := SanitizeStructuredCommandLines([]byte("\x1b[31mghcr.io\x1b[0m\n\n docker.io \x00\n"))
	want := []string{"ghcr.io", "docker.io"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected sanitized lines: got %#v want %#v", got, want)
	}
}

func TestFormatCommandOutputErrorSanitizesRuntimeNoise(t *testing.T) {
	err := FormatCommandOutputError(errors.New("exit status 1"), []byte("\x1b[31mpermission denied\x1b[0m\x00\n"))
	if err == nil {
		t.Fatalf("expected error")
	}
	if strings.Contains(err.Error(), "\x1b") || strings.Contains(err.Error(), "\x00") {
		t.Fatalf("expected sanitized error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected sanitized message, got %q", err.Error())
	}
}

func TestSanitizeJSONOutputStripsPrefixAndANSI(t *testing.T) {
	got := string(sanitizeJSONOutput([]byte("\x1b[31mwarning\x1b[0m\n[{\"name\":\"ok\"}]")))
	if got != "[{\"name\":\"ok\"}]" {
		t.Fatalf("unexpected sanitized json output: %q", got)
	}
}
