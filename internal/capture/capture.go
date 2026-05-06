package capture

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/playwright-community/playwright-go"
)

type Job struct {
	Domain string
}

// Result is emitted after a successful capture.
type Result struct {
	Domain     string
	Dir        string
	HTML       string
	Screenshot []byte
}

// Worker manages a pool of browser contexts that capture screenshots and DOMs.
type Worker struct {
	dir     string
	jobs    chan Job
	results chan Result
	pw      *playwright.Playwright
	browser playwright.Browser
	wg      sync.WaitGroup
}

// Start launches the browser and n concurrent capture goroutines.
// Captured files are saved to dir. Completed captures are sent to Results().
func Start(ctx context.Context, dir string, concurrency int) (*Worker, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright: %w", err)
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("launch chromium: %w", err)
	}

	w := &Worker{
		dir:     dir,
		jobs:    make(chan Job, 128),
		results: make(chan Result, 128),
		pw:      pw,
		browser: browser,
	}

	for range concurrency {
		w.wg.Add(1)
		go w.loop(ctx)
	}

	go func() {
		w.wg.Wait()
		close(w.results)
	}()

	return w, nil
}

// Results returns a channel of completed captures for downstream processing.
func (w *Worker) Results() <-chan Result {
	return w.results
}

// Submit queues a domain for capture. Non-blocking; drops if the queue is full.
func (w *Worker) Submit(domain string) {
	select {
	case w.jobs <- Job{Domain: domain}:
	default:
		log.Printf("capture: queue full, dropping %s", domain)
	}
}

// Stop waits for in-flight captures to finish and cleans up the browser.
func (w *Worker) Stop() {
	close(w.jobs)
	w.wg.Wait()
	w.browser.Close()
	w.pw.Stop()
}

func (w *Worker) loop(ctx context.Context) {
	defer w.wg.Done()
	for {
		select {
		case job, ok := <-w.jobs:
			if !ok {
				return
			}
			if err := w.capture(ctx, job.Domain); err != nil {
				log.Printf("capture %s: %v", job.Domain, err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) capture(ctx context.Context, domain string) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	subdir := filepath.Join(w.dir, ts+"_"+id.String())
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", subdir, err)
	}
	stem := filepath.Join(subdir, domain)

	page, err := w.browser.NewPage(playwright.BrowserNewPageOptions{
		IgnoreHttpsErrors: playwright.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("new page: %w", err)
	}
	defer page.Close()

	url := "https://" + domain
	resp, err := page.Goto(url, playwright.PageGotoOptions{
		Timeout:   playwright.Float(10000),
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		return fmt.Errorf("goto %s: %w", url, err)
	}
	log.Printf("capture: %s -> %d %s", domain, resp.Status(), stem)

	if _, err := page.Screenshot(playwright.PageScreenshotOptions{
		Path:     playwright.String(stem + ".png"),
		FullPage: playwright.Bool(true),
	}); err != nil {
		return fmt.Errorf("screenshot: %w", err)
	}

	html, err := page.Content()
	if err != nil {
		return fmt.Errorf("content: %w", err)
	}
	if err := os.WriteFile(stem+".html", []byte(html), 0o644); err != nil {
		return fmt.Errorf("write html: %w", err)
	}

	screenshot, _ := os.ReadFile(stem + ".png")
	w.results <- Result{
		Domain:     domain,
		Dir:        subdir,
		HTML:       html,
		Screenshot: screenshot,
	}

	return nil
}
