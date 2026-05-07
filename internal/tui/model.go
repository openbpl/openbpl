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
	buckets   [chartBuckets]int
	bucketIdx int
	bucketAcc int // accumulator for current second
	startedAt time.Time

	// captured domains: domain -> capture directory.
	// Populated for every successful capture, not only flagged ones, so the
	// "open in finder" key works on any row whose capture has completed.
	captureDirs map[string]string

	// capturesRoot is the parent directory that holds per-domain capture
	// subdirs; used as a fallback when a specific row hasn't been captured yet.
	capturesRoot string

	// flaggedSet tracks domains that have been flagged by rules, so new
	// detections of already-flagged domains show the badge immediately.
	flaggedSet map[string]bool

	// filter state
	showOnlyFlagged bool

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
	capturesRoot string,
) model {
	columns := []table.Column{
		{Title: "TIME", Width: 10},
		{Title: "KIND", Width: 8},
		{Title: "KEYWORD", Width: 14},
		{Title: "FLAG", Width: 4},
		{Title: "DOMAIN", Width: 44},
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
		Foreground(lipgloss.Color("#F97316")). // orange-500
		BorderLeft(true).
		BorderStyle(lipgloss.Border{Left: "→"}).
		BorderForeground(lipgloss.Color("#F97316")).
		Padding(0, 1)

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithHeight(10),
		table.WithFocused(true),
		table.WithStyles(s),
	)

	return model{
		table:        t,
		startedAt:    time.Now(),
		captureDirs:  make(map[string]string),
		flaggedSet:   make(map[string]bool),
		capturesRoot: capturesRoot,
		detectionCh:  detCh,
		captureCh:    capCh,
		ruleCh:       ruleCh,
		errCh:        errCh,
	}
}

// selectedDomain returns the domain from the currently selected table row.
func (m model) selectedDomain() (string, bool) {
	row := m.table.SelectedRow()
	if len(row) < 5 {
		return "", false
	}
	return row[4], true
}
