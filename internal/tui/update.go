package tui

import (
	"net/url"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
)

// spinnerInterval controls how often the status-line spinner advances.
// Keep it short (~10 fps) so the spinner reads as "live" without burning CPU.
const spinnerInterval = 100 * time.Millisecond

func spinnerTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

func bucketTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return bucketTickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		spinnerTick(),
		bucketTick(),
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
		case "o":
			if domain, ok := m.selectedDomain(); ok {
				// Strip wildcard prefix from CT entries so the URL is valid.
				host := strings.TrimPrefix(domain, "*.")
				browserlingURL := "https://www.browserling.com/browse/win10/chrome131/https%3A%2F%2F" + url.PathEscape(host)
				_ = exec.Command("open", browserlingURL).Start()
			}
			return m, nil
		case "f":
			target := m.capturesRoot
			if domain, ok := m.selectedDomain(); ok {
				host := strings.TrimPrefix(domain, "*.")
				if dir, captured := m.captureDirs[host]; captured && dir != "" {
					target = dir
				}
			}
			if target != "" {
				_ = exec.Command("open", target).Start()
			}
			return m, nil
		case "t":
			m.showOnlyFlagged = !m.showOnlyFlagged
			m.rebuildTableRows()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil

	case spinnerTickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		return m, spinnerTick()

	case bucketTickMsg:
		m.bucketIdx = (m.bucketIdx + 1) % chartBuckets
		m.buckets[m.bucketIdx] = m.bucketAcc
		m.bucketAcc = 0
		return m, bucketTick()

	case DetectionMsg:
		flag := ""
		if _, ok := m.flaggedSet[msg.Domain]; ok {
			flag = "⚑"
		}
		row := table.Row{
			msg.SeenAt.Format("15:04:05"),
			msg.Kind,
			msg.Keyword,
			flag,
			msg.Domain,
		}
		m.rows = append([]table.Row{row}, m.rows...)
		if len(m.rows) > maxRows {
			m.rows = m.rows[:maxRows]
		}
		m.rebuildTableRows()
		m.detections++
		m.bucketAcc++
		m.lastDomain = msg.Domain
		return m, waitForDetection(m.detectionCh)

	case CaptureMsg:
		m.captures++
		if msg.Result.Dir != "" {
			m.captureDirs[msg.Result.Domain] = msg.Result.Dir
		}
		return m, waitForCapture(m.captureCh)

	case RuleMsg:
		m.flagged += len(msg.Labels)
		if msg.Dir != "" {
			m.captureDirs[msg.Domain] = msg.Dir
		}
		m.flaggedSet[msg.Domain] = true
		// Update the flag badge on any existing rows for this domain.
		for i, row := range m.rows {
			if len(row) > 4 && row[4] == msg.Domain {
				m.rows[i][3] = "⚑"
			}
		}
		m.rebuildTableRows()
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

func (m *model) rebuildTableRows() {
	if m.showOnlyFlagged {
		var filtered []table.Row
		for _, r := range m.rows {
			if len(r) > 3 && r[3] == "⚑" {
				filtered = append(filtered, r)
			}
		}
		m.table.SetRows(filtered)
	} else {
		m.table.SetRows(m.rows)
	}
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
	const timeW, kindW, kwW, flagW = 10, 8, 14, 4
	fixedW := timeW + kindW + kwW + flagW + 5*2
	domainW := m.width - fixedW - 2
	if domainW < 20 {
		domainW = 20
	}
	m.table.SetColumns([]table.Column{
		{Title: "TIME", Width: timeW},
		{Title: "KIND", Width: kindW},
		{Title: "KEYWORD", Width: kwW},
		{Title: "FLAG", Width: flagW},
		{Title: "DOMAIN", Width: domainW},
	})
}
