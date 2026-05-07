package tui

import (
	"time"

	"github.com/openbpl/openbpl/internal/capture"
	"github.com/openbpl/openbpl/internal/rule"
)

// DetectionMsg is sent when a keyword match is found in the CertStream.
type DetectionMsg struct {
	Domain  string
	Keyword string
	Kind    string // "substr" or "lev=N"
	SeenAt  time.Time
}

// CaptureMsg is sent when a domain capture completes.
type CaptureMsg struct {
	Result capture.Result
}

// RuleMsg is sent when the rule engine produces labels for a captured domain.
type RuleMsg struct {
	Domain string
	Labels []rule.Label
}

// LogMsg carries a general log line (from redirected log.Printf).
type LogMsg string

// ErrorMsg carries a fatal error from the pipeline.
type ErrorMsg struct{ Err error }

// tickMsg is sent every second to advance the spinner and update the chart.
type tickMsg time.Time
