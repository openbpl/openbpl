package tui

import (
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

const (
	maxRows      = 1000
	chartBuckets = 60 // 60 seconds of history
)

type model struct {
	table  table.Model
	width  int
	height int
	rows   []table.Row

	// stats
	detections int
	captures   int
	flagged    int

	// spinner
	spinnerFrame int
	lastDomain   string // most recent detection for the status line

	// rate chart: detections per second, rolling window
	buckets    [chartBuckets]int
	bucketIdx  int
	bucketAcc  int // accumulator for current second
	startedAt  time.Time

	// channels
	detectionCh <-chan DetectionMsg
	captureCh   <-chan CaptureMsg
	ruleCh      <-chan RuleMsg
	errCh       <-chan error

	quitting bool
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func newModel(
	detCh <-chan DetectionMsg,
	capCh <-chan CaptureMsg,
	ruleCh <-chan RuleMsg,
	errCh <-chan error,
) model {
	columns := []table.Column{
		{Title: "TIME", Width: 10},
		{Title: "KIND", Width: 8},
		{Title: "KEYWORD", Width: 14},
		{Title: "DOMAIN", Width: 48},
	}

	s := table.DefaultStyles()
	s.Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#6B7280")). // gray-500
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#374151")) // gray-700
	s.Cell = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D1D5DB")). // gray-300
		Padding(0, 1)
	s.Selected = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#111827")).  // gray-900
		Background(lipgloss.Color("#F97316")). // orange-500
		Padding(0, 1)

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithHeight(10),
		table.WithFocused(true),
		table.WithStyles(s),
	)

	return model{
		table:       t,
		startedAt:   time.Now(),
		detectionCh: detCh,
		captureCh:   capCh,
		ruleCh:      ruleCh,
		errCh:       errCh,
	}
}
