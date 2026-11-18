package runtime

import (
	"reflect"
	"testing"
)

func TestNormalizeUlimits(t *testing.T) {
	limits, err := NormalizeUlimits([]string{
		"nofile=1024:2048",
		"nproc=512",
		"nofile=2048:4096",
	})
	if err != nil {
		t.Fatalf("NormalizeUlimits error: %v", err)
	}
	want := []string{"nofile=2048:4096", "nproc=512"}
	if !reflect.DeepEqual(limits, want) {
		t.Fatalf("unexpected normalized ulimits:\nwant: %#v\ngot:  %#v", want, limits)
	}
}

func TestNormalizeUlimitsRejectsInvalidValues(t *testing.T) {
	cases := []string{
		"nofile",
		"=1024",
		"no file=1024",
		"nofile=a",
		"nofile=200:100",
		"nofile=10:20:30",
	}
	for _, value := range cases {
		if _, err := NormalizeUlimits([]string{value}); err == nil {
			t.Fatalf("expected error for %q", value)
		}
	}
}
