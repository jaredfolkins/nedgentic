package scripts

import (
	"strings"
	"testing"
)

func TestRunnerTemplatesDoNotRecursivelyChownWorkspaceParent(t *testing.T) {
	data := RunnerData{
		UserName:       "'alice'",
		UserUID:        501,
		UserGID:        20,
		HomeDir:        "'/home/alice'",
		ShellPath:      "'/bin/sh'",
		GenticDir:      "'/ned/hooks'",
		ShellCommand:   "'/bin/sh'",
		WorkspacePath:  "'/home/alice/tmp'",
		WorkspaceMount: "'/mnt/ned-workspaces/Users/alice/tmp'",
	}

	for name, render := range map[string]func(RunnerData) (string, error){
		"runner":    RenderRunner,
		"bootstrap": RenderBootstrap,
	} {
		t.Run(name, func(t *testing.T) {
			text, err := render(data)
			if err != nil {
				t.Fatalf("render %s: %v", name, err)
			}
			if strings.Contains(text, `chown -R "$USER_UID":"$USER_GID" "$WS_PARENT"`) {
				t.Fatalf("%s template recursively chowns workspace parent:\n%s", name, text)
			}
			if !strings.Contains(text, `chown "$USER_UID":"$USER_GID" "$WS_PARENT" 2>/dev/null || true`) {
				t.Fatalf("%s template does not chown only the workspace parent:\n%s", name, text)
			}
		})
	}
}
