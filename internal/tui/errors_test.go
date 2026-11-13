package tui

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/iRootPro/lofi-player/internal/radiobrowser"
)

func TestDiscoveryErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"timeout", context.DeadlineExceeded, "Radio Browser is taking too long. Try again."},
		{"rate limit", &radiobrowser.HTTPStatusError{StatusCode: http.StatusTooManyRequests}, "Radio Browser is limiting requests. Try again shortly."},
		{"server", &radiobrowser.HTTPStatusError{StatusCode: http.StatusBadGateway}, "Radio Browser is temporarily unavailable. Try again later."},
		{"bad response", radiobrowser.ErrInvalidResponse, "Radio Browser sent an invalid response. Try again."},
		{"network", &net.DNSError{Err: "no such host", Name: "internal.example", IsNotFound: true}, "Can't reach Radio Browser. Check your connection."},
		{"unknown", errors.New("sensitive internal details"), "Radio Browser is unavailable. Try again later."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := discoveryErrorMessage(tt.err); got != tt.want {
				t.Errorf("discoveryErrorMessage(%v) = %q, want %q", tt.err, got, tt.want)
			}
			if got := discoveryErrorMessage(tt.err); got == tt.err.Error() {
				t.Errorf("discoveryErrorMessage leaked raw error %q", got)
			}
		})
	}
}
