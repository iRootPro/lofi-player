package tui

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/iRootPro/lofi-player/internal/radiobrowser"
)

// discoveryErrorMessage turns transport/API failures into calm, actionable UI
// copy. The original errors remain wrapped for callers and tests, but never
// appear in the terminal interface.
func discoveryErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Radio Browser is taking too long. Try again."
	}
	var statusErr *radiobrowser.HTTPStatusError
	if errors.As(err, &statusErr) {
		switch {
		case statusErr.StatusCode == http.StatusTooManyRequests:
			return "Radio Browser is limiting requests. Try again shortly."
		case statusErr.StatusCode >= http.StatusInternalServerError:
			return "Radio Browser is temporarily unavailable. Try again later."
		default:
			return "Radio Browser could not complete this search. Try again."
		}
	}
	if errors.Is(err, radiobrowser.ErrInvalidResponse) {
		return "Radio Browser sent an invalid response. Try again."
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) {
		if networkErr.Timeout() {
			return "Radio Browser is taking too long. Try again."
		}
		return "Can't reach Radio Browser. Check your connection."
	}
	return "Radio Browser is unavailable. Try again later."
}
