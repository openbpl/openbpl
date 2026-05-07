package tui

import (
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
)

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickEvery(),
		waitForDetection(m.detectionCh),
		waitForCapture(m.captureCh),
		waitForRule(m.ruleCh),
		waitForError(m.errCh),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil

	case tickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		// Rotate bucket.
		m.bucketIdx = (m.bucketIdx + 1) % chartBuckets
		m.buckets[m.bucketIdx] = m.bucketAcc
		m.bucketAcc = 0
		return m, tickEvery()

	case DetectionMsg:
		row := table.Row{
			msg.SeenAt.Format("15:04:05"),
			msg.Kind,
			msg.Keyword,
			msg.Domain,
		}
		m.rows = append([]table.Row{row}, m.rows...)
		if len(m.rows) > maxRows {
			m.rows = m.rows[:maxRows]
		}
		m.table.SetRows(m.rows)
		m.detections++
		m.bucketAcc++
		m.lastDomain = msg.Domain
		return m, waitForDetection(m.detectionCh)

	case CaptureMsg:
		m.captures++
		return m, waitForCapture(m.captureCh)

	case RuleMsg:
		m.flagged += len(msg.Labels)
		return m, waitForRule(m.ruleCh)

	case LogMsg:
		return m, nil

	case ErrorMsg:
		return m, waitForError(m.errCh)
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *model) resize() {
	if m.width == 0 || m.height == 0 {
		return
	}
	// Layout: header(3) + chart(chartHeight+1) + table(rest) + status(1)
	chartH := 8
	chrome := 3 + chartH + 1 + 1 // header + chart + gap + status
	tableH := m.height - chrome
	if tableH < 3 {
		tableH = 3
	}

	m.table.SetWidth(m.width)
	m.table.SetHeight(tableH)

	// Dynamic column widths.
	const timeW, kindW, kwW = 10, 8, 14
	fixedW := timeW + kindW + kwW + 4*2
	domainW := m.width - fixedW - 2
	if domainW < 20 {
		domainW = 20
	}
	m.table.SetColumns([]table.Column{
		{Title: "TIME", Width: timeW},
		{Title: "KIND", Width: kindW},
		{Title: "KEYWORD", Width: kwW},
		{Title: "DOMAIN", Width: domainW},
	})
}
