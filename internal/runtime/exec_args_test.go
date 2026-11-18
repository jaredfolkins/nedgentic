package runtime

import (
	"reflect"
	"testing"
)

func TestBuildExecArgsWithTTYUserEnvWorkdir(t *testing.T) {
	uid := 501
	gid := 20
	args, err := BuildExecArgs(ExecOptions{
		TTY:           true,
		UID:           &uid,
		GID:           &gid,
		Env:           []string{"HOME=/home/jf", "USER=jf"},
		Ulimits:       []string{"nofile=1024:2048"},
		Workdir:       "/home/jf",
		ContainerName: "ned-default",
		Command:       []string{"/bin/sh", "-c", "echo hi"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"exec",
		"-it",
		"--uid", "501",
		"--gid", "20",
		"--env", "HOME=/home/jf",
		"--env", "USER=jf",
		"--ulimit", "nofile=1024:2048",
		"--workdir", "/home/jf",
		"ned-default",
		"/bin/sh", "-c", "echo hi",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args:\nwant: %#v\ngot:  %#v", want, args)
	}
}

func TestBuildExecArgsWithoutTTY(t *testing.T) {
	args, err := BuildExecArgs(ExecOptions{
		TTY:           false,
		ContainerName: "ned-default",
		Command:       []string{"ls", "-la"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"exec", "ned-default", "ls", "-la"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args:\nwant: %#v\ngot:  %#v", want, args)
	}
}

func TestBuildExecArgsErrors(t *testing.T) {
	_, err := BuildExecArgs(ExecOptions{
		TTY:     false,
		Command: []string{"ls"},
	})
	if err == nil {
		t.Fatalf("expected error for missing container name")
	}
	_, err = BuildExecArgs(ExecOptions{
		TTY:           false,
		ContainerName: "ned-default",
	})
	if err == nil {
		t.Fatalf("expected error for missing command")
	}
	_, err = BuildExecArgs(ExecOptions{
		TTY:           false,
		ContainerName: "ned-default",
		Command:       []string{"ls"},
		Ulimits:       []string{"nofile"},
	})
	if err == nil {
		t.Fatalf("expected error for invalid ulimit")
	}
}
