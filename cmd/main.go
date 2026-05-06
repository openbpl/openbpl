package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/openbpl/openbpl/internal/sources"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	entries, errs := sources.Stream(ctx, sources.DefaultCertstreamURL)

	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				return
			}
			fmt.Printf("[%s] index=%d domains=%v\n", entry.Seen.Format("15:04:05"), entry.CertIndex, entry.AllDomains)
		case err := <-errs:
			if err != nil {
				log.Fatal(err)
			}
			return
		}
	}
}
