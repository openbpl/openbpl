package tui

import (
	tea "charm.land/bubbletea/v2"
)

// waitForDetection blocks on ch and returns a DetectionMsg.
func waitForDetection(ch <-chan DetectionMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// waitForCapture blocks on the capture channel and returns a CaptureMsg.
func waitForCapture(ch <-chan CaptureMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// waitForRule blocks on ch and returns a RuleMsg.
func waitForRule(ch <-chan RuleMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// waitForError blocks on ch and returns an ErrorMsg.
func waitForError(ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-ch
		if !ok {
			return nil
		}
		return ErrorMsg{Err: err}
	}
}
