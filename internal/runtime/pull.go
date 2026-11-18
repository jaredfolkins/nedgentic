package runtime

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// ManagedPull downloads image anonymously and loads it into the container
// runtime, which avoids keychain prompts for public images.
func ManagedPull(bin, image, platform string) error {
	ref, err := parseImageReference(image)
	if err != nil {
		return err
	}

	fmt.Printf("Downloading image %s...\n", ref.Name())
	options := []remote.Option{
		remote.WithPlatform(defaultPullPlatform(platform)),
	}
	img, err := remote.Image(ref, options...)
	if err != nil {
		if isManifestUnknown(err) {
			return fmt.Errorf("image not found (manifest unknown): %s; if this is a Ned image, build it with its Makefile under containers/ (for example `make -C containers/arm64/minimal`)", ref.Name())
		}
		return fmt.Errorf("download failed: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "ned-pull-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	layoutDir := filepath.Join(tmpDir, "layout")
	if err := os.MkdirAll(layoutDir, 0700); err != nil {
		return err
	}
	path, err := layout.Write(layoutDir, empty.Index)
	if err != nil {
		return fmt.Errorf("write OCI layout: %w", err)
	}
	annotations := map[string]string{
		"org.opencontainers.image.ref.name": ref.Name(),
	}
	if err := path.AppendImage(img, layout.WithAnnotations(annotations)); err != nil {
		return fmt.Errorf("append OCI image: %w", err)
	}

	tarPath := filepath.Join(tmpDir, "image.tar")
	if err := tarDirectory(layoutDir, tarPath); err != nil {
		return fmt.Errorf("archive OCI layout: %w", err)
	}

	fmt.Println("Loading image into container runtime...")
	if err := ContainerImageLoad(bin, tarPath); err != nil {
		return fmt.Errorf("load image: %w", err)
	}
	if image != "" && image != ref.Name() {
		if err := ContainerImageTag(bin, ref.Name(), image); err != nil {
			return fmt.Errorf("tag image: %w", err)
		}
	}
	return nil
}

func isManifestUnknown(err error) bool {
	var terr *transport.Error
	if errors.As(err, &terr) {
		for _, diagnostic := range terr.Errors {
			if diagnostic.Code == transport.ManifestUnknownErrorCode {
				return true
			}
		}
	}
	return strings.Contains(err.Error(), string(transport.ManifestUnknownErrorCode))
}

func parseImageReference(image string) (name.Reference, error) {
	return name.ParseReference(image, name.WithDefaultRegistry("docker.io"), name.WithDefaultTag("latest"))
}

func defaultPullPlatform(override string) v1.Platform {
	if override != "" {
		return parsePlatform(override)
	}
	arch := stdruntime.GOARCH
	switch arch {
	case "amd64", "arm64":
		return v1.Platform{OS: "linux", Architecture: arch}
	default:
		return v1.Platform{OS: "linux", Architecture: arch}
	}
}

func parsePlatform(s string) v1.Platform {
	parts := strings.Split(s, "/")
	p := v1.Platform{OS: "linux", Architecture: "amd64"} // defaults
	if len(parts) > 0 {
		if len(parts) == 1 {
			p.Architecture = parts[0]
		} else {
			p.OS = parts[0]
			p.Architecture = parts[1]
			if len(parts) > 2 {
				p.Variant = parts[2]
			}
		}
	}
	return p
}

func tarDirectory(srcDir, tarPath string) error {
	outFile, err := os.OpenFile(tarPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer outFile.Close()

	writer := tar.NewWriter(outFile)
	defer writer.Close()

	return filepath.WalkDir(srcDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if entry.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if entry.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(writer, file); err != nil {
			file.Close()
			return err
		}
		return file.Close()
	})
}
