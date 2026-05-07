package tui

import (
	"context"
	"encoding/base64"
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

	"github.com/openbpl/openbpl/internal/bridge"
	"github.com/openbpl/openbpl/internal/capture"
	"github.com/openbpl/openbpl/internal/config"
	"github.com/openbpl/openbpl/internal/detect"
	"github.com/openbpl/openbpl/internal/notify"
	"github.com/openbpl/openbpl/internal/rule"
	"github.com/openbpl/openbpl/internal/sdk"
	"github.com/openbpl/openbpl/internal/sources"
	"github.com/openbpl/openbpl/internal/store"
)

// capturesRoot is the on-disk directory where per-domain capture subdirs live.
const capturesRoot = "data"

// Run initialises the pipeline and launches the TUI.
func Run() error {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open("detections.db")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	// Ensure the embedded SDK is extracted into node_modules/.
	if err := sdk.Ensure("."); err != nil {
		return fmt.Errorf("extract sdk: %w", err)
	}

	// Start the Node.js rule sidecar if rules/ directory exists.
	rulesDir, _ := filepath.Abs("rules")
	runtimePath := filepath.Join("node_modules", "@openbpl", "sdk", "dist", "runtime.js")
	nodeBridge, err := bridge.Start(runtimePath, rulesDir)
	if err != nil {
		return fmt.Errorf("start rules engine: %w", err)
	}
	defer nodeBridge.Stop()

	cap, err := capture.Start(ctx, capturesRoot, 3)
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

			screenshotPath := filepath.Join(res.Dir, res.Domain+".png")
			params := bridge.EvaluateParams{
				Evidence: bridge.Evidence{
					Domain:         res.Domain,
					HTML:           res.HTML,
					Title:          extractTitle(res.HTML),
					ScreenshotPath: screenshotPath,
					Screenshot:     base64.StdEncoding.EncodeToString(res.Screenshot),
				},
				Brand: bridge.Brand{
					Name:        cfg.Brand.Name,
					Website:     cfg.Brand.Website,
					Description: cfg.Brand.Description,
					Industry:    cfg.Brand.Industry,
					Keywords: bridge.Keywords{
						Included: cfg.Brand.Keywords.Included,
						Excluded: cfg.Brand.Keywords.Excluded,
					},
					Images: cfg.Brand.Images,
					Colors: cfg.Brand.Colors,
					URLs: bridge.URLs{
						Domains:           cfg.Brand.URLs.Domains,
						SocialMedia:       cfg.Brand.URLs.SocialMedia,
						AppStores:         cfg.Brand.URLs.AppStores,
						BrowserExtensions: cfg.Brand.URLs.BrowserExtensions,
						Blogs:             cfg.Brand.URLs.Blogs,
					},
				},
			}

			bridgeLabels, err := nodeBridge.Evaluate(params)
			if err != nil {
				log.Printf("rule evaluate: %v", err)
				continue
			}

			if len(bridgeLabels) > 0 {
				// Convert bridge labels to rule.Label for compatibility
				labels := make([]rule.Label, len(bridgeLabels))
				for i, bl := range bridgeLabels {
					labels[i] = rule.Label{
						Rule:       bl.Name,
						Name:       bl.Name,
						Confidence: bl.Confidence,
						Detail:     bl.Detail,
					}
				}

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
	matcher := detect.New(cfg.Brand.Keywords.Included, cfg.Brand.Keywords.Excluded, 1)
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
	m := newModel(detCh, captureCh, ruleCh, errCh, capturesRoot)
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

// extractTitle returns the content of the <title> tag from HTML.
func extractTitle(html string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title")
	if start == -1 {
		return ""
	}
	start = strings.Index(lower[start:], ">")
	if start == -1 {
		return ""
	}
	start += strings.Index(lower, "<title") + 1
	end := strings.Index(lower[start:], "</title>")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(html[start : start+end])
}
