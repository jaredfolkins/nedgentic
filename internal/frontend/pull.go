package frontend

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
	"nedgentic/internal/release"
)

// releaseImageTimeout bounds the default image download and load.
const releaseImageTimeout = 30 * time.Minute

// minimalImageRepo is the local repository of Ned's own minimal image; it is
// never pulled from a registry.
const minimalImageRepo = "ned-minimal"

var (
	containerImageExistsForPull = runtime.ContainerImageExists
	managedPullFunc             = runtime.ManagedPull
	fetchReleaseImageFunc       = fetchReleaseImage
	defaultReleaseImage         = release.DefaultImage
)

// ensureImagePull makes imageRef available in the container runtime. The
// default image comes from this ned release's GitHub asset; other ned-minimal
// tags must be built locally; everything else is a registry pull.
func ensureImagePull(bin, imageRef, platform string) error {
	exists, err := containerImageExistsForPull(bin, imageRef)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if !isMinimalImageRef(imageRef) {
		return managedPullFunc(bin, imageRef, platform)
	}
	img, ok, err := defaultReleaseImage()
	if err != nil {
		return err
	}
	if ok && sameImageRef(imageRef, img.Ref) {
		return fetchReleaseImageFunc(bin, img)
	}
	if ok {
		return fmt.Errorf("image %s is not available locally; this ned release ships %s (set \"image\" in the profile to it), or build %s with `make -C containers/arm64/minimal TAG=%s`", imageRef, img.Ref, imageRef, imageTag(imageRef))
	}
	return fmt.Errorf("image %s is not available locally; dev builds of ned do not download it. Build it with `make -C containers/arm64/minimal` (tags %s), or use a release build of ned", imageRef, release.DevImageRef)
}

// fetchReleaseImage downloads, verifies, and loads the release image with a
// progress line and an overall timeout.
func fetchReleaseImage(bin string, img release.Image) error {
	home, err := profile.ResolveHome()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), releaseImageTimeout)
	defer cancel()
	output.Printf("Download: %s from %s", img.Ref, img.URL)
	progress := newDownloadProgress("Download: "+img.Ref, runtime.WantsTTY())
	err = runtime.FetchReleaseImage(ctx, bin, img, profile.BaseDir(home), progress.update)
	progress.finish(err)
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("downloading %s timed out after %s", img.Ref, releaseImageTimeout)
	}
	return err
}

// isMinimalImageRef reports whether ref names the local ned-minimal repository
// (bare, or as the runtime normalizes it under docker.io/library).
func isMinimalImageRef(ref string) bool {
	repo := strings.TrimSpace(ref)
	if at := strings.Index(repo, "@"); at >= 0 {
		repo = repo[:at]
	}
	if slash := strings.LastIndex(repo, "/"); slash >= 0 {
		if colon := strings.LastIndex(repo, ":"); colon > slash {
			repo = repo[:colon]
		}
	} else if colon := strings.LastIndex(repo, ":"); colon >= 0 {
		repo = repo[:colon]
	}
	for _, prefix := range []string{"docker.io/library/", "index.docker.io/library/"} {
		repo = strings.TrimPrefix(repo, prefix)
	}
	return repo == minimalImageRepo
}

func imageTag(ref string) string {
	if colon := strings.LastIndex(ref, ":"); colon >= 0 && !strings.Contains(ref[colon:], "/") {
		return ref[colon+1:]
	}
	return "latest"
}

func sameImageRef(a, b string) bool {
	return isMinimalImageRef(a) && isMinimalImageRef(b) && imageTag(a) == imageTag(b)
}

// downloadProgress prints "<label>... 42% (120/290 MB)" in place on a TTY and
// a line per quarter otherwise.
type downloadProgress struct {
	mu          sync.Mutex
	label       string
	interactive bool
	last        time.Time
	lastQuarter int64
}

func newDownloadProgress(label string, interactive bool) *downloadProgress {
	return &downloadProgress{label: label, interactive: interactive}
}

func (p *downloadProgress) update(done, total int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.interactive {
		if time.Since(p.last) < 200*time.Millisecond && done != total {
			return
		}
		p.last = time.Now()
		_, _ = fmt.Fprintf(output.StderrWriter(), "\r\033[K%s", output.Format(p.label+"... "+formatProgress(done, total)))
		return
	}
	if total <= 0 {
		return
	}
	quarter := done * 4 / total
	if quarter > p.lastQuarter {
		p.lastQuarter = quarter
		output.Print(p.label + "... " + formatProgress(done, total))
	}
}

func (p *downloadProgress) finish(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.interactive {
		_, _ = fmt.Fprint(output.StderrWriter(), "\r\033[K")
	}
	if err != nil {
		output.Print(p.label + "... failed")
		return
	}
	output.Print(p.label + "... done")
}

func formatProgress(done, total int64) string {
	const mb = 1 << 20
	if total <= 0 {
		return fmt.Sprintf("%d MB", done/mb)
	}
	return fmt.Sprintf("%d%% (%d/%d MB)", done*100/total, done/mb, total/mb)
}
