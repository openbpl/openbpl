package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/openbpl/openbpl/internal/capture"
	"github.com/openbpl/openbpl/internal/detect"
	"github.com/openbpl/openbpl/internal/notify"
	"github.com/openbpl/openbpl/internal/rule"
	"github.com/openbpl/openbpl/internal/sources"
	"github.com/openbpl/openbpl/internal/store"
)

var keywords = []string{
	"coinbase",
	"metamask",
	"paypal",
	"binance",
	"kraken",
	"ledger",
	"trezor",
}

// Run initialises the pipeline and launches the TUI.
func Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open("detections.db")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	engine := rule.NewEngine()
	faviconRule, err := rule.NewFaviconMatch("rules/favicons", 5)
	if err != nil {
		log.Printf("favicon rule disabled: %v", err)
	} else {
		engine.Register(faviconRule)
	}
	engine.Register(&rule.LoginFormDetector{})

	cap, err := capture.Start(ctx, "data", 3)
	if err != nil {
		return fmt.Errorf("start capture: %w", err)
	}
	defer cap.Stop()

	flaggedFile, err := os.OpenFile("flagged.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open flagged.txt: %w", err)
	}
	defer flaggedFile.Close()

	// Channels between pipeline goroutines and the TUI.
	detCh := make(chan DetectionMsg, 256)
	captureCh := make(chan CaptureMsg, 64)
	ruleCh := make(chan RuleMsg, 64)
	errCh := make(chan error, 1)

	// --- Rule processing goroutine ---
	// Reads from cap.Results() and fans out to captureCh + ruleCh.
	go func() {
		for res := range cap.Results() {
			// Notify TUI of completed capture.
			select {
			case captureCh <- CaptureMsg{Result: res}:
			default:
			}

			ev := rule.Evidence{
				Domain:     res.Domain,
				HTML:       res.HTML,
				Screenshot: res.Screenshot,
			}
			labels := engine.Run(ev)
			if len(labels) > 0 {
				labelsJSON, _ := json.MarshalIndent(labels, "", "  ")
				_ = os.WriteFile(filepath.Join(res.Dir, "labels.json"), labelsJSON, 0o644)

				for _, l := range labels {
					line := fmt.Sprintf("[%s] domain=%s rule=%s label=%s confidence=%.2f %s dir=%s\n",
						time.Now().UTC().Format(time.RFC3339),
						res.Domain, l.Rule, l.Name, l.Confidence, l.Detail, res.Dir)
					fmt.Fprint(flaggedFile, line)
				}

				screenshot := filepath.Join(res.Dir, res.Domain+".png")
				notify.Send(res.Domain, labels, screenshot)

				select {
				case ruleCh <- RuleMsg{Domain: res.Domain, Dir: res.Dir, Labels: labels}:
				default:
				}
			}
		}
	}()

	// --- CertStream detection goroutine ---
	matcher := detect.New(keywords, 1)
	entries, streamErrs := sources.Stream(ctx, sources.DefaultCertstreamURL)

	go func() {
		defer close(detCh)
		for {
			select {
			case entry, ok := <-entries:
				if !ok {
					return
				}
				hits := matcher.Check(entry.AllDomains)
				captured := make(map[string]bool)
				for _, h := range hits {
					kind := "substr"
					if !h.Exact {
						kind = fmt.Sprintf("lev=%d", h.Distance)
					}
					if err := db.Insert(store.Detection{
						Domain:   h.Domain,
						Keyword:  h.Keyword,
						Kind:     kind,
						Distance: h.Distance,
						SeenAt:   entry.Seen,
					}); err != nil {
						log.Printf("db insert: %v", err)
					}

					select {
					case detCh <- DetectionMsg{
						Domain:  h.Domain,
						Keyword: h.Keyword,
						Kind:    kind,
						SeenAt:  entry.Seen,
					}:
					default:
					}

					domain := strings.TrimPrefix(h.Domain, "*.")
					if !captured[domain] {
						captured[domain] = true
						cap.Submit(domain)
					}
				}
			case err := <-streamErrs:
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
				}
				return
			}
		}
	}()

	// --- Launch the TUI ---
	m := newModel(detCh, captureCh, ruleCh, errCh)
	p := tea.NewProgram(m)

	// Redirect all log.* output into the TUI log panel.
	lw := newLogWriter(p)
	log.SetOutput(lw)
	log.SetFlags(0)

	// Suppress raw stderr (Playwright, etc.) from corrupting the TUI.
	os.Stderr = os.NewFile(0, os.DevNull)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	stop()
	return nil
}
