package frontend

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"nedgentic/internal/output"
)

func TestRenderOutputGoldenJSON(t *testing.T) {
	restore := output.Snapshot()
	t.Cleanup(func() { output.Configure(restore) })

	tests := []struct {
		name   string
		mode   output.Mode
		value  any
		golden string
	}{
		{
			name: "vm list json",
			mode: output.ModeJSON,
			value: listResponse[listRow]{
				Kind: "vm_list",
				Items: []listRow{
					{
						Name:      "alpha",
						Profile:   "default",
						Image:     "docker.io/library/alpine",
						Status:    "running",
						Hostname:  "alpha",
						IP:        "10.0.0.2",
						Workspace: "/home/alice/work",
						User:      "alice",
					},
				},
				Summary: storageSummary{
					RuntimeDataRoot: "/Users/alice/Library/Application Support/com.apple.container",
				},
			},
			golden: "testdata/output/vm_list.json.golden",
		},
		{
			name: "profile list json",
			mode: output.ModeJSON,
			value: listResponse[profileListItem]{
				Kind:  "profile_list",
				Items: []profileListItem{{Name: "default"}},
			},
			golden: "testdata/output/profile_list.json.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output.Configure(output.Config{
				Mode:        tt.mode,
				Stdout:      &bytes.Buffer{},
				Stderr:      &bytes.Buffer{},
				StdoutIsTTY: false,
				StderrIsTTY: false,
			})

			cmd := &cobra.Command{Use: "test"}
			var out bytes.Buffer
			cmd.SetOut(&out)

			if err := renderOutput(cmd, tt.value, nil); err != nil {
				t.Fatalf("renderOutput error: %v", err)
			}

			want, err := os.ReadFile(tt.golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if out.String() != string(want) {
				t.Fatalf("unexpected output:\nwant:\n%s\ngot:\n%s", string(want), out.String())
			}
		})
	}
}
