package tui

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/iRootPro/lofi-player/internal/i18n"
	"github.com/iRootPro/lofi-player/internal/radiobrowser"
)

// discoveryErrorMessage turns transport/API failures into calm, actionable UI
// copy. The original errors remain wrapped for callers and tests, but never
// appear in the terminal interface.
func discoveryErrorMessage(err error) string {
	return discoveryErrorMessageFor(i18n.EN, err)
}

func discoveryErrorMessageFor(locale i18n.Locale, err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return i18n.T(locale, "rb_slow")
	}
	var statusErr *radiobrowser.HTTPStatusError
	if errors.As(err, &statusErr) {
		switch {
		case statusErr.StatusCode == http.StatusTooManyRequests:
			return i18n.T(locale, "rb_limited")
		case statusErr.StatusCode >= http.StatusInternalServerError:
			return i18n.T(locale, "rb_temporary")
		default:
			return i18n.T(locale, "rb_search_failed")
		}
	}
	if errors.Is(err, radiobrowser.ErrInvalidResponse) {
		return i18n.T(locale, "rb_invalid")
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) {
		if networkErr.Timeout() {
			return i18n.T(locale, "rb_slow")
		}
		return i18n.T(locale, "rb_connection")
	}
	return i18n.T(locale, "rb_unavailable")
}
