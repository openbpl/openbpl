package rule

import (
	"regexp"
	"strings"
)

var passwordInputRe = regexp.MustCompile(`(?i)<input[^>]+type=["']password["'][^>]*>`)
var emailInputRe = regexp.MustCompile(`(?i)<input[^>]+type=["'](?:email|text)["'][^>]*name=["'](?:email|user|login|username)["'][^>]*>`)
var formActionRe = regexp.MustCompile(`(?i)<form[^>]+action=["']([^"']+)["'][^>]*>`)

// LoginFormDetector flags pages that contain credential harvesting forms.
type LoginFormDetector struct{}

func (l *LoginFormDetector) Name() string { return "login-form" }

func (l *LoginFormDetector) Evaluate(ev Evidence) ([]Label, error) {
	html := strings.ToLower(ev.HTML)
	var labels []Label

	hasPassword := passwordInputRe.MatchString(html)
	hasEmail := emailInputRe.MatchString(html)

	if !hasPassword {
		return nil, nil
	}

	confidence := 0.5
	detail := "password input detected"
	if hasEmail {
		confidence = 0.8
		detail = "login form with email/username + password inputs"
	}

	if m := formActionRe.FindStringSubmatch(ev.HTML); len(m) > 1 {
		action := m[1]
		if action != "" && !strings.Contains(action, ev.Domain) {
			confidence = 0.95
			detail += "; form posts to external domain: " + action
		}
	}

	labels = append(labels, Label{
		Rule:       l.Name(),
		Name:       "login-form",
		Confidence: confidence,
		Detail:     detail,
	})

	return labels, nil
}
