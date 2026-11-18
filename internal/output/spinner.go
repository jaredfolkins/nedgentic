package output

import (
	"fmt"
	"time"
)

type SpinnerOptions struct {
	Interactive   bool
	EmitFinalLine bool
}

func RunWithSpinner(label string, options SpinnerOptions, fn func() error) error {
	stderr := StderrWriter()
	if !options.Interactive {
		err := fn()
		if !options.EmitFinalLine {
			return err
		}
		if err != nil {
			_, _ = fmt.Fprintln(stderr, Format(label+"... failed"))
			return err
		}
		_, _ = fmt.Fprintln(stderr, Format(label+"... done"))
		return nil
	}
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()
	frames := []rune{'|', '/', '-', '\\'}
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	index := 0
	for {
		select {
		case err := <-done:
			if !options.EmitFinalLine {
				_, _ = fmt.Fprint(stderr, "\r\033[K")
				return err
			}
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "\r%s\n", Format(label+"... failed"))
				return err
			}
			_, _ = fmt.Fprintf(stderr, "\r%s\n", Format(label+"... done"))
			return nil
		case <-ticker.C:
			frame := frames[index%len(frames)]
			_, _ = fmt.Fprintf(stderr, "\r%s", Format(fmt.Sprintf("%s... %c", label, frame)))
			index++
		}
	}
}
