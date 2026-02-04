package auth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser opens the specified URL in the user's default browser.
// It supports macOS, Linux, and Windows.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// OpenBrowserWithFallback attempts to open the browser and returns
// instructions if it fails.
func OpenBrowserWithFallback(url string) (opened bool, fallbackMsg string) {
	if err := OpenBrowser(url); err != nil {
		return false, fmt.Sprintf("Please open this URL in your browser:\n  %s", url)
	}
	return true, ""
}
