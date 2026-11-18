package runtime

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"nedgentic/internal/profile"
)

func TestWriteDependenciesState(t *testing.T) {
	home := t.TempDir()
	state := DependenciesState{
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Dependencies: []DependencyState{
			{
				Name:      "container",
				Required:  true,
				Installed: true,
				CheckedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	if err := WriteDependenciesState(home, state); err != nil {
		t.Fatalf("WriteDependenciesState error: %v", err)
	}
	path := profile.DependenciesPath(home)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	var decoded DependenciesState
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(decoded.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(decoded.Dependencies))
	}
	if decoded.Dependencies[0].Name != "container" {
		t.Fatalf("expected dependency container, got %q", decoded.Dependencies[0].Name)
	}
}

func TestMissingRequiredDependencies(t *testing.T) {
	state := DependenciesState{
		Dependencies: []DependencyState{
			{Name: "container", Required: true, Installed: false},
			{Name: "optional", Required: false, Installed: false},
		},
	}
	missing := MissingRequiredDependencies(state)
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing dependency, got %d", len(missing))
	}
	if missing[0].Name != "container" {
		t.Fatalf("expected missing container, got %q", missing[0].Name)
	}
}
