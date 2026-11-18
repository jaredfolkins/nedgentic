package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nedgentic/internal/release"
)

// fakeContainerBin writes a stand-in `container` binary that records its
// arguments, copies loaded archives aside, and answers `image inspect` once an
// archive was loaded (or never, when loadsImage is false).
func fakeContainerBin(t *testing.T, loadsImage bool) (bin, dir string) {
	t.Helper()
	dir = t.TempDir()
	marker := filepath.Join(dir, "loaded")
	if !loadsImage {
		marker = filepath.Join(dir, "never")
	}
	script := `#!/bin/sh
echo "$*" >> "` + filepath.Join(dir, "calls") + `"
case "$1 $2" in
  "image load")
    ls -l "$4" | cut -c1-10 > "` + filepath.Join(dir, "mode") + `"
    cp "$4" "` + filepath.Join(dir, "archive") + `"
    touch "` + filepath.Join(dir, "loaded") + `"
    ;;
  "image inspect") [ -f "` + marker + `" ] ;;
  *) exit 2 ;;
esac
`
	bin = filepath.Join(dir, "container")
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return bin, dir
}

func serveArchive(t *testing.T, body []byte, status int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func sumHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func readCalls(t *testing.T, dir string) string {
	t.Helper()
	calls, err := os.ReadFile(filepath.Join(dir, "calls"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(calls)
}

func requireNoTempArchives(t *testing.T, tmpDir string) {
	t.Helper()
	left, _ := filepath.Glob(filepath.Join(tmpDir, "ned-image-*"))
	if len(left) != 0 {
		t.Fatalf("temp archives left behind: %v", left)
	}
}

func TestFetchReleaseImageVerifiesThenLoads(t *testing.T) {
	body := []byte("fake oci archive")
	server := serveArchive(t, body, http.StatusOK)
	bin, dir := fakeContainerBin(t, true)
	tmpDir := filepath.Join(t.TempDir(), ".ned")
	var lastDone, lastTotal int64
	img := release.Image{Ref: "ned-minimal:v1.2.3", URL: server.URL + "/ned-minimal-arm64.oci.tar.gz", SHA256: strings.ToUpper(sumHex(body))}
	err := FetchReleaseImage(context.Background(), bin, img, tmpDir, func(done, total int64) { lastDone, lastTotal = done, total })
	if err != nil {
		t.Fatalf("FetchReleaseImage: %v", err)
	}
	calls := readCalls(t, dir)
	if !strings.Contains(calls, "image load --input "+tmpDir+"/ned-image-") || !strings.Contains(calls, "image inspect ned-minimal:v1.2.3") {
		t.Fatalf("unexpected container calls:\n%s", calls)
	}
	loaded, err := os.ReadFile(filepath.Join(dir, "archive"))
	if err != nil || string(loaded) != string(body) {
		t.Fatalf("loaded archive = %q, %v", loaded, err)
	}
	if mode, _ := os.ReadFile(filepath.Join(dir, "mode")); strings.TrimSpace(string(mode)) != "-rw-------" {
		t.Fatalf("temp archive mode = %q, want -rw-------", mode)
	}
	if info, err := os.Stat(tmpDir); err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("temp dir: %v %v", info, err)
	}
	if lastDone != int64(len(body)) || lastTotal != int64(len(body)) {
		t.Fatalf("progress = %d/%d, want %d/%d", lastDone, lastTotal, len(body), len(body))
	}
	requireNoTempArchives(t, tmpDir)
}

func TestFetchReleaseImageChecksumMismatchNeverLoads(t *testing.T) {
	server := serveArchive(t, []byte("tampered archive"), http.StatusOK)
	bin, dir := fakeContainerBin(t, true)
	tmpDir := t.TempDir()
	img := release.Image{Ref: "ned-minimal:v1.2.3", URL: server.URL, SHA256: sumHex([]byte("real archive"))}
	err := FetchReleaseImage(context.Background(), bin, img, tmpDir, nil)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	if calls := readCalls(t, dir); calls != "" {
		t.Fatalf("container must not run on a bad checksum, got:\n%s", calls)
	}
	requireNoTempArchives(t, tmpDir)
}

func TestFetchReleaseImageFailures(t *testing.T) {
	body := []byte("archive")
	for _, tc := range []struct {
		name       string
		status     int
		sha        string
		loadsImage bool
		wantErr    string
		wantLoad   bool
	}{
		{name: "http error", status: http.StatusNotFound, sha: sumHex(body), loadsImage: true, wantErr: "404"},
		{name: "bad sha format", status: http.StatusOK, sha: "abc", loadsImage: true, wantErr: "64 hex"},
		{name: "ref missing after load", status: http.StatusOK, sha: sumHex(body), loadsImage: false, wantErr: "is missing", wantLoad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := serveArchive(t, body, tc.status)
			bin, dir := fakeContainerBin(t, tc.loadsImage)
			tmpDir := t.TempDir()
			img := release.Image{Ref: "ned-minimal:v1.2.3", URL: server.URL, SHA256: tc.sha}
			err := FetchReleaseImage(context.Background(), bin, img, tmpDir, nil)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
			if loaded := strings.Contains(readCalls(t, dir), "image load"); loaded != tc.wantLoad {
				t.Fatalf("image load called = %v, want %v", loaded, tc.wantLoad)
			}
			requireNoTempArchives(t, tmpDir)
		})
	}
}

func TestFetchReleaseImageHonorsContext(t *testing.T) {
	server := serveArchive(t, []byte("archive"), http.StatusOK)
	bin, dir := fakeContainerBin(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	img := release.Image{Ref: "ned-minimal:v1.2.3", URL: server.URL, SHA256: sumHex([]byte("archive"))}
	if err := FetchReleaseImage(ctx, bin, img, t.TempDir(), nil); err == nil {
		t.Fatalf("expected a canceled download to fail")
	}
	if calls := readCalls(t, dir); calls != "" {
		t.Fatalf("container must not run after a failed download:\n%s", calls)
	}
}
