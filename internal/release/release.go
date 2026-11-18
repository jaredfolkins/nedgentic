// Package release holds the build's release identity. make release embeds it
// with -ldflags -X; dev builds leave it empty.
package release

import (
	"fmt"
	"regexp"
	"strings"
)

// DevImageRef is the default image for dev builds: the tag
// `make -C containers/arm64/minimal` builds.
const DevImageRef = "ned-minimal:dev"

// Set by make release (see the top-level Makefile).
var (
	// Version is the release tag, e.g. v0.1.0.
	Version = ""
	// ImageRef is the local name of the default image, e.g. ned-minimal:v0.1.0.
	ImageRef = ""
	// ImageURL is the GitHub release asset holding the image archive.
	ImageURL = ""
	// ImageSHA256 is the hex sha256 of the archive at ImageURL.
	ImageSHA256 = ""
)

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Image is the default image as a downloadable release asset.
type Image struct {
	Ref    string
	URL    string
	SHA256 string
}

// DefaultImageRef is the image new profiles start from.
func DefaultImageRef() string {
	if ref := strings.TrimSpace(ImageRef); ref != "" {
		return ref
	}
	return DevImageRef
}

// DefaultImage returns the release asset for the default image. ok is false
// for dev builds; err is set when the embedded release info is incomplete or
// malformed, so a broken build fails closed instead of downloading.
func DefaultImage() (img Image, ok bool, err error) {
	img = Image{
		Ref:    strings.TrimSpace(ImageRef),
		URL:    strings.TrimSpace(ImageURL),
		SHA256: strings.ToLower(strings.TrimSpace(ImageSHA256)),
	}
	if img.Ref == "" && img.URL == "" && img.SHA256 == "" {
		return Image{}, false, nil
	}
	if img.Ref == "" || img.URL == "" || img.SHA256 == "" {
		return Image{}, false, fmt.Errorf("this ned build has incomplete release image info (ref=%q url=%q sha256=%q)", img.Ref, img.URL, img.SHA256)
	}
	if !strings.HasPrefix(img.URL, "https://") {
		return Image{}, false, fmt.Errorf("release image URL must use https: %s", img.URL)
	}
	if !sha256Hex.MatchString(img.SHA256) {
		return Image{}, false, fmt.Errorf("release image sha256 is not 64 hex characters: %q", img.SHA256)
	}
	return img, true, nil
}
