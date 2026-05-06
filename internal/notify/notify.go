package notify

import (
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
)

// Send posts a macOS notification, opens the capture directory in Finder,
// and opens the domain in Browserling for safe remote browsing.
func Send(title, body, domain, screenshotPath string) {
	absPath, _ := filepath.Abs(screenshotPath)

	script := fmt.Sprintf(
		`display notification %s with title %s`,
		appleQuote(body),
		appleQuote(title),
	)
	_ = exec.Command("osascript", "-e", script).Run()

	dir := filepath.Dir(absPath)
	_ = exec.Command("open", dir).Run()

	browserlingURL := "https://www.browserling.com/browse/win10/chrome131/https%3A%2F%2F" + url.PathEscape(domain)
	_ = exec.Command("open", browserlingURL).Run()
}

func appleQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
