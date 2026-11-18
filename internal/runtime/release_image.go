package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"nedgentic/internal/release"
)

// releaseImageTempPattern names the archive while it downloads.
const releaseImageTempPattern = "ned-image-*.oci.tar.gz"

var releaseHTTPClient = http.DefaultClient

// DownloadProgress reports bytes received so far and the total (-1 if the
// server did not send a length).
type DownloadProgress func(done, total int64)

// FetchReleaseImage downloads the release image archive into a private temp
// file under tmpDir, verifies its sha256 before anything else touches it,
// loads it with `container image load`, and checks that img.Ref is then
// present. The temp file is always removed; a checksum mismatch never loads.
func FetchReleaseImage(ctx context.Context, bin string, img release.Image, tmpDir string, progress DownloadProgress) error {
	want := strings.ToLower(strings.TrimSpace(img.SHA256))
	if len(want) != sha256.Size*2 {
		return fmt.Errorf("release image sha256 must be %d hex characters", sha256.Size*2)
	}
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(tmpDir, releaseImageTempPattern)
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}

	got, err := downloadTo(ctx, img.URL, file, progress)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("download %s: %w", img.URL, err)
	}
	if got != want {
		return fmt.Errorf("release image checksum mismatch for %s: got sha256 %s, want %s; the download was deleted and nothing was loaded", img.URL, got, want)
	}

	if err := ContainerImageLoad(bin, path); err != nil {
		return fmt.Errorf("load image: %w", err)
	}
	exists, err := containerImageExists(bin, img.Ref)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("release archive loaded but image %s is missing", img.Ref)
	}
	return nil
}

// downloadTo streams url into w and returns the hex sha256 of the body.
func downloadTo(ctx context.Context, url string, w io.Writer, progress DownloadProgress) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := releaseHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	hash := sha256.New()
	counter := &progressWriter{total: resp.ContentLength, report: progress}
	if _, err := io.Copy(io.MultiWriter(w, hash, counter), resp.Body); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

type progressWriter struct {
	done   int64
	total  int64
	report DownloadProgress
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.report != nil {
		p.report(p.done, p.total)
	}
	return len(b), nil
}
