package frontend

import (
	"slices"
	"testing"

	"github.com/spf13/pflag"
)

func TestRootCommandsAreTheCoreSet(t *testing.T) {
	var names []string
	for _, cmd := range rootCmd.Commands() {
		names = append(names, cmd.Name())
	}
	slices.Sort(names)
	want := []string{"desktop", "exec", "gentic", "list", "remove", "start", "stop"}
	if !slices.Equal(names, want) {
		t.Fatalf("unexpected commands: got %v want %v", names, want)
	}
	if rootCmd.HasAvailablePersistentFlags() {
		t.Fatal("ned must not define global flags")
	}
}

func TestRemoveHasAlias(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"remove"})
	if err != nil {
		t.Fatalf("find remove: %v", err)
	}
	if !slices.Contains(cmd.Aliases, "rm") {
		t.Fatalf("expected remove to have rm alias")
	}
}

func TestShouldSkipDependencyCheck(t *testing.T) {
	if !shouldSkipDependencyCheck(rootCmd) {
		t.Fatalf("expected root to skip dependency check")
	}
	if shouldSkipDependencyCheck(genticCmd) {
		t.Fatalf("did not expect gentic to skip dependency check")
	}
}

func TestGenticTakesProfileDirectly(t *testing.T) {
	if genticCmd.Args(genticCmd, nil) == nil {
		t.Fatal("ned gentic must require a profile")
	}
	if err := genticCmd.Args(genticCmd, []string{"default", "work-a"}); err != nil {
		t.Fatalf("ned gentic <profile> [vm] rejected: %v", err)
	}
	if genticCmd.HasSubCommands() {
		t.Fatal("ned gentic must not have subcommands")
	}
	var flags []string
	genticCmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name != "help" {
			flags = append(flags, f.Name)
		}
	})
	if !slices.Equal(flags, []string{"herdr"}) {
		t.Fatalf("unexpected gentic flags: %v", flags)
	}
}
