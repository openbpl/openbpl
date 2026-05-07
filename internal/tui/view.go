package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── palette ──────────────────────────────────────────────────────────────────

var (
	orange  = lipgloss.Color("#F97316")
	white   = lipgloss.Color("#F9FAFB")
	gray50  = lipgloss.Color("#F9FAFB")
	gray300 = lipgloss.Color("#D1D5DB")
	gray400 = lipgloss.Color("#9CA3AF")
	gray500 = lipgloss.Color("#6B7280")
	gray600 = lipgloss.Color("#4B5563")
	gray700 = lipgloss.Color("#374151")
	gray800 = lipgloss.Color("#1F2937")
	gray900 = lipgloss.Color("#111827")
)

func (m model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	w := m.width
	if w == 0 {
		w = 80
	}

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
		m.viewHeader(w),
		m.viewChart(w),
		"",
		m.table.View(),
		m.viewStatus(w),
	))
	v.AltScreen = true
	return v
}

// ── header ───────────────────────────────────────────────────────────────────

func (m model) viewHeader(w int) string {
	logo := lipgloss.NewStyle().
		Bold(true).
		Foreground(gray900).
		Background(orange).
		Padding(0, 1).
		Render("openbpl")

	elapsed := time.Since(m.startedAt).Truncate(time.Second)

	metrics := lipgloss.NewStyle().Foreground(gray500).Render(
		fmt.Sprintf("  %s %s   %s %s   %s %s   %s %s",
			lipgloss.NewStyle().Foreground(gray400).Render("detections"),
			lipgloss.NewStyle().Bold(true).Foreground(white).Render(fmt.Sprintf("%d", m.detections)),
			lipgloss.NewStyle().Foreground(gray400).Render("captures"),
			lipgloss.NewStyle().Bold(true).Foreground(gray300).Render(fmt.Sprintf("%d", m.captures)),
			lipgloss.NewStyle().Foreground(gray400).Render("flagged"),
			lipgloss.NewStyle().Bold(true).Foreground(orange).Render(fmt.Sprintf("%d", m.flagged)),
			lipgloss.NewStyle().Foreground(gray600).Render("uptime"),
			lipgloss.NewStyle().Foreground(gray500).Render(elapsed.String()),
		),
	)

	line := logo + metrics
	padW := w - lipgloss.Width(line)
	if padW < 0 {
		padW = 0
	}

	return lipgloss.NewStyle().
		Background(gray900).
		Width(w).
		Render(line + strings.Repeat(" ", padW))
}

// ── sparkline chart ──────────────────────────────────────────────────────────

// Braille-inspired block characters for the sparkline (8 levels).
var barChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

func (m model) viewChart(w int) string {
	// Collect buckets in chronological order, then append the in-progress
	// current second so the rightmost bar reacts immediately to new detections
	// instead of waiting for the next bucket rotation.
	vals := make([]int, 0, chartBuckets+1)
	for i := range chartBuckets {
		idx := (m.bucketIdx + 1 + i) % chartBuckets
		vals = append(vals, m.buckets[idx])
	}
	vals = append(vals, m.bucketAcc)

	// Find max for scaling.
	maxVal := 1
	for _, v := range vals {
		if v > maxVal {
			maxVal = v
		}
	}

	// Determine chart width (use available width minus label).
	label := " det/s "
	chartW := w - len(label) - 2
	if chartW < 10 {
		chartW = 10
	}
	if chartW > len(vals) {
		chartW = len(vals)
	}

	// Take the last chartW values.
	start := len(vals) - chartW
	if start < 0 {
		start = 0
	}
	visible := vals[start:]

	// Build the sparkline.
	var sb strings.Builder
	for _, v := range visible {
		level := v * 7 / maxVal
		if level > 7 {
			level = 7
		}
		if v == 0 {
			sb.WriteString(" ")
		} else {
			sb.WriteString(barChars[level])
		}
	}

	sparkline := lipgloss.NewStyle().
		Foreground(orange).
		Render(sb.String())

	labelStr := lipgloss.NewStyle().
		Foreground(gray600).
		Render(label)

	maxLabel := lipgloss.NewStyle().
		Foreground(gray700).
		Render(fmt.Sprintf(" ↑%d", maxVal))

	return labelStr + sparkline + maxLabel
}

// ── status bar ───────────────────────────────────────────────────────────────

func (m model) viewStatus(w int) string {
	spinner := lipgloss.NewStyle().
		Foreground(orange).
		Render(spinnerFrames[m.spinnerFrame])

	var activity string
	if m.lastDomain != "" {
		domain := m.lastDomain
		maxLen := w - 30
		if maxLen > 0 && len(domain) > maxLen {
			domain = domain[:maxLen] + "…"
		}
		activity = lipgloss.NewStyle().
			Foreground(gray400).
			Render(domain)
	} else {
		activity = lipgloss.NewStyle().
			Foreground(gray600).
			Render("waiting for detections…")
	}

	left := spinner + " " + activity

	help := lipgloss.NewStyle().
		Foreground(gray700).
		Render("↑↓ scroll  o browser  f finder  q quit")

	padW := w - lipgloss.Width(left) - lipgloss.Width(help)
	if padW < 1 {
		padW = 1
	}

	return lipgloss.NewStyle().
		Background(gray900).
		Width(w).
		Render(left + strings.Repeat(" ", padW) + help)
}
