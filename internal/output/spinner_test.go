package output

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

type synchronizedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (buf *synchronizedBuffer) Write(p []byte) (int, error) {
	buf.mu.Lock()
	defer buf.mu.Unlock()
	return buf.buf.Write(p)
}

func (buf *synchronizedBuffer) String() string {
	buf.mu.Lock()
	defer buf.mu.Unlock()
	return buf.buf.String()
}

func TestRunWithSpinnerInteractiveRendersAndClearsStep(t *testing.T) {
	previous := Snapshot()
	t.Cleanup(func() { Configure(previous) })

	var stderr synchronizedBuffer
	Configure(Config{
		Mode:        ModeText,
		Stdout:      &bytes.Buffer{},
		Stderr:      &stderr,
		StdoutIsTTY: false,
		StderrIsTTY: false,
	})

	started := make(chan struct{})
	release := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- RunWithSpinner("Wait for ready", SpinnerOptions{
			Interactive:   true,
			EmitFinalLine: false,
		}, func() error {
			close(started)
			<-release
			return nil
		})
	}()

	<-started
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(stderr.String(), "\rWait for ready... |") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatalf("RunWithSpinner error: %v", err)
	}

	got := stderr.String()
	if !strings.Contains(got, "\rWait for ready... |") {
		t.Fatalf("expected an interactive spinner frame, got %q", got)
	}
	if !strings.HasSuffix(got, "\r\033[K") {
		t.Fatalf("expected the interactive step to clear its line, got %q", got)
	}
}
