package frontend

import "slices"

import "testing"

func TestShouldSkipRuntimeCheck(t *testing.T) {
	if !shouldSkipRuntimeCheck(runtimeInstallCmd) {
		t.Fatalf("expected runtime install to skip runtime check")
	}
	if !shouldSkipRuntimeCheck(runtimeRemoveCmd) {
		t.Fatalf("expected runtime remove to skip runtime check")
	}
	if !shouldSkipRuntimeCheck(runtimeCmd) {
		t.Fatalf("expected runtime to skip runtime check")
	}
	if shouldSkipRuntimeCheck(genticCmd) {
		t.Fatalf("did not expect gentic to skip runtime check")
	}
	if shouldSkipRuntimeCheck(rootCmd) {
		t.Fatalf("did not expect root to skip runtime check")
	}
}

func TestRemoveHasAlias(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "remove" {
			continue
		}
		found = true
		if slices.Contains(cmd.Aliases, "rm") {
			return
		}
		t.Fatalf("expected remove to have rm alias")
	}
	if !found {
		t.Fatalf("expected remove command")
	}
}

func TestShouldSkipDependencyCheck(t *testing.T) {
	if !shouldSkipDependencyCheck(runtimeInstallCmd) {
		t.Fatalf("expected runtime install to skip dependency check")
	}
	if !shouldSkipDependencyCheck(rootCmd) {
		t.Fatalf("expected root to skip dependency check")
	}
	if shouldSkipDependencyCheck(genticCmd) {
		t.Fatalf("did not expect gentic to skip dependency check")
	}
}

func TestImageBuildPushAndRegistryLoginAreRemoved(t *testing.T) {
	for _, path := range [][]string{{"build"}, {"login"}, {"logout"}, {"registry"}, {"image", "push"}, {"image", "login"}, {"image", "logout"}} {
		if _, err := resolveCommandPath(rootCmd, path); err == nil {
			t.Fatalf("ned must not offer `ned %v`", path)
		}
	}
	for _, path := range [][]string{{"image", "pull"}, {"image", "list"}} {
		if _, err := resolveCommandPath(rootCmd, path); err != nil {
			t.Fatalf("expected `ned %v`: %v", path, err)
		}
	}
	if rootCmd.PersistentFlags().Lookup("build-pull") != nil {
		t.Fatal("ned must not offer --build-pull")
	}
}

func TestSystemProfileScopeIsRemoved(t *testing.T) {
	if _, err := resolveCommandPath(rootCmd, []string{"gentic", "system"}); err == nil {
		t.Fatal("ned must not offer `ned gentic system`")
	}
	if _, err := resolveCommandPath(rootCmd, []string{"gentic", "personal"}); err != nil {
		t.Fatalf("expected `ned gentic personal`: %v", err)
	}
}

func TestHostExecIsRemoved(t *testing.T) {
	if _, err := resolveCommandPath(rootCmd, []string{"hostexec"}); err == nil {
		t.Fatal("ned must not offer `ned hostexec`")
	}
	for _, name := range []string{"host-exec", "host-exec-bin", "host-exec-allow-ned"} {
		if genticCmd.PersistentFlags().Lookup(name) != nil {
			t.Fatalf("ned gentic must not offer --%s", name)
		}
	}
	if genticCmd.PersistentFlags().Lookup("herdr") == nil {
		t.Fatal("expected --herdr on ned gentic")
	}
}
