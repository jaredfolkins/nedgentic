package frontend

import (
	"errors"
	"testing"
)

func TestShouldPrintRuntimeInstallHint(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "already contains install guidance",
			err:  errors.New("requires container 0.10.0 or newer. Upgrade with `ned runtime install`"),
			want: false,
		},
		{
			name: "generic missing runtime error",
			err:  errors.New("container runtime binary \"container\" not found"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldPrintRuntimeInstallHint(tt.err); got != tt.want {
				t.Fatalf("shouldPrintRuntimeInstallHint() = %v, want %v", got, tt.want)
			}
		})
	}
}
