package tui

import (
	"bytes"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// logWriter captures log output and sends it into the Bubble Tea program
// as LogMsg messages. It implements io.Writer.
type logWriter struct {
	p   *tea.Program
	mu  sync.Mutex
	buf bytes.Buffer
}

func newLogWriter(p *tea.Program) *logWriter {
	return &logWriter{p: p}
}

func (w *logWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(b)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// Incomplete line — put it back.
			if len(line) > 0 {
				w.buf.WriteString(line)
			}
			break
		}
		line = strings.TrimRight(line, "\n\r")
		if line != "" {
			w.p.Send(LogMsg(line))
		}
	}
	return len(b), nil
}
