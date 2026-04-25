// Package notify sends OS-level desktop notifications.
//
// Notifications are best-effort: on platforms where no native mechanism
// is available (or the helper command is missing), Send returns nil and
// the caller is expected to fall back to an in-app Toast.
package notify

import (
	"os/exec"
	"runtime"
)

// Send fires a desktop notification with the given title and body.
// Always returns nil — notification failures should never bubble up
// to the user.
func Send(title, body string) error {
	switch runtime.GOOS {
	case "darwin":
		sendMacOS(title, body)
	case "linux":
		sendLinux(title, body)
	}
	return nil
}

func sendMacOS(title, body string) {
	// AppleScript injection guard: the strings flow into a script
	// literal, so any embedded double-quote breaks the call. Sanitize.
	t := escapeForAppleScript(title)
	b := escapeForAppleScript(body)
	script := `display notification "` + b + `" with title "` + t + `"`
	_ = exec.Command("osascript", "-e", script).Run()
}

func sendLinux(title, body string) {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return
	}
	_ = exec.Command("notify-send", title, body).Run()
}

// escapeForAppleScript replaces double-quotes and backslashes that
// would otherwise terminate the AppleScript string literal.
func escapeForAppleScript(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\\' {
			out = append(out, '\\')
		}
		out = append(out, c)
	}
	return string(out)
}
