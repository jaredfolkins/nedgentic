package release

import (
	"strings"
	"testing"
)

func setRelease(t *testing.T, ref, url, sha string) {
	t.Helper()
	origRef, origURL, origSHA := ImageRef, ImageURL, ImageSHA256
	t.Cleanup(func() { ImageRef, ImageURL, ImageSHA256 = origRef, origURL, origSHA })
	ImageRef, ImageURL, ImageSHA256 = ref, url, sha
}

func TestDefaultImageDevBuild(t *testing.T) {
	setRelease(t, "", "", "")
	if _, ok, err := DefaultImage(); ok || err != nil {
		t.Fatalf("dev build: ok=%v err=%v", ok, err)
	}
	if got := DefaultImageRef(); got != DevImageRef {
		t.Fatalf("DefaultImageRef() = %q, want %q", got, DevImageRef)
	}
}

func TestDefaultImageRelease(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	url := "https://github.com/jaredfolkins/nedgentic/releases/download/v1.0.0/ned-minimal-arm64.oci.tar.gz"
	setRelease(t, "ned-minimal:v1.0.0", url, strings.ToUpper(sha))
	img, ok, err := DefaultImage()
	if !ok || err != nil {
		t.Fatalf("release build: ok=%v err=%v", ok, err)
	}
	if img.Ref != "ned-minimal:v1.0.0" || img.URL != url || img.SHA256 != sha {
		t.Fatalf("unexpected image %+v", img)
	}
}

func TestDefaultImageRejectsBrokenReleaseInfo(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	for name, tc := range map[string][3]string{
		"missing sha": {"ned-minimal:v1", "https://example.com/a", ""},
		"http url":    {"ned-minimal:v1", "http://example.com/a", sha},
		"short sha":   {"ned-minimal:v1", "https://example.com/a", "abcd"},
	} {
		t.Run(name, func(t *testing.T) {
			setRelease(t, tc[0], tc[1], tc[2])
			if _, ok, err := DefaultImage(); ok || err == nil {
				t.Fatalf("expected an error, got ok=%v err=%v", ok, err)
			}
		})
	}
}
