package main

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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open("detections.db")
	if err != nil {
		log.Fatalf("open db: %v", err)
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
		log.Fatalf("start capture: %v", err)
	}
	defer cap.Stop()

	flaggedFile, err := os.OpenFile("flagged.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Fatalf("open flagged.txt: %v", err)
	}
	defer flaggedFile.Close()

	// Run rules on completed captures in a separate goroutine.
	go func() {
		for res := range cap.Results() {
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
					log.Printf("rule [%s] domain=%s label=%s confidence=%.2f %s",
						l.Rule, res.Domain, l.Name, l.Confidence, l.Detail)
				}

				screenshot := filepath.Join(res.Dir, res.Domain+".png")
				notify.Send(res.Domain, labels, screenshot)
			}
		}
	}()

	matcher := detect.New(keywords, 1)
	entries, errs := sources.Stream(ctx, sources.DefaultCertstreamURL)

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
				fmt.Printf("[%s] [%s] keyword=%s domain=%s domains=%s\n",
					entry.Seen.Format("15:04:05"),
					kind,
					h.Keyword,
					h.Domain,
					strings.Join(entry.AllDomains, ", "),
				)
				if err := db.Insert(store.Detection{
					Domain:   h.Domain,
					Keyword:  h.Keyword,
					Kind:     kind,
					Distance: h.Distance,
					SeenAt:   entry.Seen,
				}); err != nil {
					log.Printf("db insert: %v", err)
				}
				domain := strings.TrimPrefix(h.Domain, "*.")
				if !captured[domain] {
					captured[domain] = true
					cap.Submit(domain)
				}
			}
		case err := <-errs:
			if err != nil {
				log.Fatal(err)
			}
			return
		}
	}
}
