package runtime

import (
	"errors"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

func TestIsManifestUnknown(t *testing.T) {
	err := &transport.Error{
		Errors: []transport.Diagnostic{
			{Code: transport.ManifestUnknownErrorCode, Message: "manifest unknown"},
		},
	}
	if !isManifestUnknown(err) {
		t.Fatalf("expected manifest unknown for transport error")
	}

	err = &transport.Error{
		Errors: []transport.Diagnostic{
			{Code: transport.NameUnknownErrorCode, Message: "name unknown"},
		},
	}
	if isManifestUnknown(err) {
		t.Fatalf("expected manifest unknown to be false for different code")
	}

	if !isManifestUnknown(errors.New("MANIFEST_UNKNOWN")) {
		t.Fatalf("expected manifest unknown to be true for fallback string match")
	}

	if isManifestUnknown(errors.New("NAME_UNKNOWN")) {
		t.Fatalf("expected manifest unknown to be false for other errors")
	}
}
