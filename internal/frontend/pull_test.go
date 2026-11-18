package frontend

import (
	"errors"
	"strings"
	"testing"

	"nedgentic/internal/release"
)

func TestEnsureImagePullPullsMissingImage(t *testing.T) {
	restore := capturePullDeps()
	t.Cleanup(restore)

	containerImageExistsForPull = func(bin, image string) (bool, error) {
		return false, nil
	}
	var pulled string
	managedPullFunc = func(bin, image, platform string) error {
		pulled = image
		return nil
	}

	if err := ensureImagePull("container", "docker.io/example/app:latest", ""); err != nil {
		t.Fatalf("ensureImagePull error: %v", err)
	}
	if pulled != "docker.io/example/app:latest" {
		t.Fatalf("expected managed pull of the image, got %q", pulled)
	}
}

func TestEnsureImagePullSkipsWhenImageExists(t *testing.T) {
	restore := capturePullDeps()
	t.Cleanup(restore)

	containerImageExistsForPull = func(bin, image string) (bool, error) {
		return true, nil
	}
	managedPullFunc = func(bin, image, platform string) error {
		t.Fatalf("unexpected managed pull")
		return nil
	}

	if err := ensureImagePull("container", "docker.io/example/app:latest", ""); err != nil {
		t.Fatalf("ensureImagePull error: %v", err)
	}
}

func capturePullDeps() func() {
	origImageExists := containerImageExistsForPull
	origManagedPull := managedPullFunc
	origFetch := fetchReleaseImageFunc
	origDefault := defaultReleaseImage

	return func() {
		containerImageExistsForPull = origImageExists
		managedPullFunc = origManagedPull
		fetchReleaseImageFunc = origFetch
		defaultReleaseImage = origDefault
	}
}

// stubMissingImage makes every image missing and fails any registry pull.
func stubMissingImage(t *testing.T) {
	t.Helper()
	t.Cleanup(capturePullDeps())
	containerImageExistsForPull = func(bin, image string) (bool, error) { return false, nil }
	managedPullFunc = func(bin, image, platform string) error {
		t.Fatalf("unexpected registry pull of %s", image)
		return nil
	}
}

func TestEnsureImagePullDownloadsReleaseImage(t *testing.T) {
	stubMissingImage(t)
	want := release.Image{Ref: "ned-minimal:v1.0.0", URL: "https://example.com/a.oci.tar.gz", SHA256: strings.Repeat("a", 64)}
	defaultReleaseImage = func() (release.Image, bool, error) { return want, true, nil }
	var fetched release.Image
	fetchReleaseImageFunc = func(bin string, img release.Image) error {
		fetched = img
		return nil
	}
	for _, ref := range []string{"ned-minimal:v1.0.0", "docker.io/library/ned-minimal:v1.0.0"} {
		fetched = release.Image{}
		if err := ensureImagePull("container", ref, ""); err != nil {
			t.Fatalf("ensureImagePull(%s): %v", ref, err)
		}
		if fetched != want {
			t.Fatalf("ensureImagePull(%s) fetched %+v, want %+v", ref, fetched, want)
		}
	}
}

func TestEnsureImagePullPropagatesReleaseFetchError(t *testing.T) {
	stubMissingImage(t)
	defaultReleaseImage = func() (release.Image, bool, error) {
		return release.Image{Ref: "ned-minimal:v1.0.0"}, true, nil
	}
	fetchReleaseImageFunc = func(bin string, img release.Image) error { return errors.New("checksum mismatch") }
	if err := ensureImagePull("container", "ned-minimal:v1.0.0", ""); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected the fetch error, got %v", err)
	}
}

func TestEnsureImagePullMinimalImageWithoutRelease(t *testing.T) {
	stubMissingImage(t)
	fetchReleaseImageFunc = func(bin string, img release.Image) error {
		t.Fatalf("dev builds must not download")
		return nil
	}
	defaultReleaseImage = func() (release.Image, bool, error) { return release.Image{}, false, nil }
	err := ensureImagePull("container", "ned-minimal:dev", "")
	if err == nil || !strings.Contains(err.Error(), "make -C containers/arm64/minimal") {
		t.Fatalf("expected a build hint for dev builds, got %v", err)
	}

	// A release build asked for another ned-minimal tag points at its own.
	defaultReleaseImage = func() (release.Image, bool, error) {
		return release.Image{Ref: "ned-minimal:v2.0.0"}, true, nil
	}
	err = ensureImagePull("container", "ned-minimal:v1.0.0", "")
	if err == nil || !strings.Contains(err.Error(), "ships ned-minimal:v2.0.0") || !strings.Contains(err.Error(), "TAG=v1.0.0") {
		t.Fatalf("expected a hint naming the shipped image, got %v", err)
	}
}

func TestIsMinimalImageRef(t *testing.T) {
	for ref, want := range map[string]bool{
		"ned-minimal:dev":                       true,
		"ned-minimal":                           true,
		"docker.io/library/ned-minimal:v1.0.0":  true,
		"docker.io/jfolkins/ned-minimal:v1.0.0": false,
		"ghcr.io/example/ned-minimal:v1":        false,
		"ned-minimal-e2e:arm64":                 false,
		"localhost:5000/ned-minimal:v1":         false,
		"docker.io/library/alpine:latest":       false,
	} {
		if got := isMinimalImageRef(ref); got != want {
			t.Errorf("isMinimalImageRef(%q) = %v, want %v", ref, got, want)
		}
	}
}
