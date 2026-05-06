package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/openbpl/openbpl/internal/detect"
	"github.com/openbpl/openbpl/internal/sources"
	"github.com/openbpl/openbpl/internal/store"
)

var keywords = []string{
	"coinbase",
	"metamask",
	"paypal",
	"binance",
	"kraken",
	"blockchain",
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

	matcher := detect.New(keywords, 1)
	entries, errs := sources.Stream(ctx, sources.DefaultCertstreamURL)

	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				return
			}
			hits := matcher.Check(entry.AllDomains)
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
			}
		case err := <-errs:
			if err != nil {
				log.Fatal(err)
			}
			return
		}
	}
}
