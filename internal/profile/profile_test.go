package profile

import (
	"nedgentic/internal/release"
	"strings"
	"testing"
)

func TestTemplateParsesAsHuJSON(t *testing.T) {
	content := Template()
	var data map[string]any
	if err := UnmarshalHuJSON([]byte(content), &data); err != nil {
		t.Fatalf("template HuJSON parse failed: %v", err)
	}
}

func TestUnmarshalHuJSONRejectsUnknownField(t *testing.T) {
	input := `{
  "shell": "/bin/bash",
  "extra": "nope",
}`
	var dst struct {
		Shell string `json:"shell"`
	}
	err := UnmarshalHuJSON([]byte(input), &dst)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got: %v", err)
	}
}

func TestTemplateWithShellDefaults(t *testing.T) {
	content := TemplateWithShell(TemplateOptions{})
	if !strings.Contains(content, `"shell": "/bin/bash"`) {
		t.Fatalf("expected default shell in template, got:\n%s", content)
	}
	if !strings.Contains(content, `"image": "`+DefaultImageReference()+`"`) {
		t.Fatalf("expected default image in template, got:\n%s", content)
	}
	if strings.Contains(content, `"flags": {`) {
		t.Fatalf("did not expect default flags block in sparse template, got:\n%s", content)
	}
	if strings.Contains(content, `"imports":`) {
		t.Fatalf("did not expect default imports block in sparse template, got:\n%s", content)
	}
}

func TestTemplateWithShellOverrides(t *testing.T) {
	flags := DefaultFlags()
	flags.LogLevel = "debug"
	init := true
	systemd := true
	flags.Init = &init
	flags.InitImage = "docker.io/library/custom-init:latest"
	flags.Systemd = &systemd
	flags.CPUs = "4"
	flags.Memory = "8G"
	flags.Ulimits = []string{"nofile=1024:2048"}
	image := &ImageState{Current: "docker.io/library/ubuntu:24.04", History: map[string]string{}}
	content := TemplateWithShell(TemplateOptions{
		Shell:    "/bin/zsh",
		Flags:    flags,
		Binaries: []string{"rg"},
		Image:    image,
	})
	for _, expected := range []string{
		`"shell": "/bin/zsh"`,
		`"log-level": "debug"`,
		`"image": "docker.io/library/ubuntu:24.04"`,
		`"resources": {`,
		`"imports": {`,
		`"rg"`,
		`"cpus": "4"`,
		`"memory": "8G"`,
		`"init": true`,
		`"init-image": "docker.io/library/custom-init:latest"`,
		`"systemd": true`,
		`"nofile=1024:2048"`,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected %q in template, got:\n%s", expected, content)
		}
	}
}

func TestDefaultImageIsMinimal(t *testing.T) {
	if got, want := DefaultImageReference(), "ned-minimal:dev"; got != want {
		t.Fatalf("dev DefaultImageReference() = %q, want %q", got, want)
	}
	orig := release.ImageRef
	t.Cleanup(func() { release.ImageRef = orig })
	release.ImageRef = "ned-minimal:v1.2.3"
	if got := DefaultImageReference(); got != "ned-minimal:v1.2.3" {
		t.Fatalf("release DefaultImageReference() = %q, want ned-minimal:v1.2.3", got)
	}
}
