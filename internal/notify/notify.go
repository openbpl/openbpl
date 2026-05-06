package notify

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/openbpl/openbpl/internal/phishreport"
	"github.com/openbpl/openbpl/internal/rule"
)

// Send posts a macOS banner notification, opens Finder + Browserling,
// then writes draft abuse report emails into the capture directory.
func Send(domain string, labels []rule.Label, screenshotPath string) {
	absPath, _ := filepath.Abs(screenshotPath)
	dir := filepath.Dir(absPath)

	// Banner notification (non-blocking visual alert).
	var details []string
	for _, l := range labels {
		details = append(details, l.Detail)
	}
	body := strings.Join(details, "; ")
	bannerScript := fmt.Sprintf(
		`display notification %s with title %s`,
		appleQuote(body),
		appleQuote("openbpl: "+domain),
	)
	_ = exec.Command("osascript", "-e", bannerScript).Run()

	// Open capture folder + Browserling immediately.
	_ = exec.Command("open", dir).Run()
	browserlingURL := "https://www.browserling.com/browse/win10/chrome131/https%3A%2F%2F" + url.PathEscape(domain)
	_ = exec.Command("open", browserlingURL).Run()

	// Look up abuse contacts and write draft emails in the background.
	go writeDraftEmails(domain, labels, dir)
}

func writeDraftEmails(domain string, labels []rule.Label, captureDir string) {
	contacts, err := phishreport.Lookup(domain)
	if err != nil {
		log.Printf("phishreport: %v", err)
		return
	}
	if len(contacts) == 0 {
		return
	}

	for _, c := range pickContacts(contacts, 4) {
		draft := buildDraftEmail(c, domain, labels, captureDir)
		name := fmt.Sprintf("abuse-report-%s-%s.txt", sanitizeFilePart(c.Role), sanitizeFilePart(c.Name))
		path := filepath.Join(captureDir, name)
		if err := os.WriteFile(path, []byte(draft), 0o644); err != nil {
			log.Printf("write draft email: %v", err)
		}
	}
}

func buildDraftEmail(c phishreport.Contact, domain string, labels []rule.Label, captureDir string) string {
	// If the report_uri is already a mailto, extract the address.
	to := ""
	if strings.HasPrefix(c.ReportURI, "mailto:") {
		to = strings.TrimPrefix(c.ReportURI, "mailto:")
		if idx := strings.Index(to, "?"); idx != -1 {
			to = to[:idx]
		}
	}

	subject := fmt.Sprintf("Phishing report: %s", domain)

	var bodyLines []string
	bodyLines = append(bodyLines, fmt.Sprintf("Domain: https://%s", domain))
	bodyLines = append(bodyLines, fmt.Sprintf("Registrar/Host: %s (%s)", c.Name, c.Role))
	bodyLines = append(bodyLines, "")
	bodyLines = append(bodyLines, "Detection details:")
	for _, l := range labels {
		bodyLines = append(bodyLines, fmt.Sprintf("  - [%s] %s (confidence: %.0f%%)", l.Rule, l.Detail, l.Confidence*100))
	}
	bodyLines = append(bodyLines, "")
	absDir, _ := filepath.Abs(captureDir)
	bodyLines = append(bodyLines, fmt.Sprintf("Evidence captured at: %s", absDir))
	bodyLines = append(bodyLines, "")
	bodyLines = append(bodyLines, "This domain was flagged by openbpl (https://github.com/openbpl/openbpl).")

	var draft []string
	draft = append(draft, "To: "+to)
	draft = append(draft, "Subject: "+subject)
	if to == "" && c.ReportURI != "" {
		draft = append(draft, "Report-URI: "+c.ReportURI)
	}
	draft = append(draft, "")
	draft = append(draft, strings.Join(bodyLines, "\n"))
	draft = append(draft, "")
	return strings.Join(draft, "\n")
}

var rolePriority = map[string]int{
	"registrar": 0,
	"hosting":   1,
}

func pickContacts(contacts []phishreport.Contact, n int) []phishreport.Contact {
	// Sort by role priority, pick top n.
	type scored struct {
		contact phishreport.Contact
		score   int
	}
	var s []scored
	for _, c := range contacts {
		p, ok := rolePriority[c.Role]
		if !ok {
			p = 99
		}
		s = append(s, scored{contact: c, score: p})
	}
	// Simple selection sort (tiny list).
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j].score < s[i].score {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
	if len(s) > n {
		s = s[:n]
	}
	out := make([]phishreport.Contact, len(s))
	for i, v := range s {
		out[i] = v.contact
	}
	return out
}

func sanitizeFilePart(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}

func appleQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
