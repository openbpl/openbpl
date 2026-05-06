package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const DefaultCertstreamURL = "wss://certstream-server-production.up.railway.app"

type Entry struct {
	CertIndex  int64
	Seen       time.Time
	AllDomains []string
	LeafCert   LeafCert
}

type LeafCert struct {
	Subject     Subject
	Issuer      Subject
	NotBefore   string
	NotAfter    string
	Fingerprint string
}

// Subject holds distinguished name fields.
type Subject struct {
	CN string
	O  string
	C  string
}

// Stream connects to the certstream websocket at url and delivers certificate
// update entries to the returned channel until ctx is cancelled or a fatal
// error occurs. The error channel receives at most one value.
func Stream(ctx context.Context, url string) (<-chan Entry, <-chan error) {
	entries := make(chan Entry, 64)
	errs := make(chan error, 1)

	go func() {
		defer close(entries)
		defer close(errs)

		backoff := time.Second
		const maxBackoff = 30 * time.Second

		for {
			err := stream(ctx, url, entries)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				log.Printf("certstream: %v (reconnecting in %s)", err, backoff)
			} else {
				log.Printf("certstream: connection closed (reconnecting in %s)", backoff)
			}

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}

			backoff = min(backoff*2, maxBackoff)
		}
	}()

	return entries, errs
}

// ── internal ──────────────────────────────────────────────────────────────────

func stream(ctx context.Context, url string, out chan<- Entry) error {
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", url, err)
	}
	defer conn.Close()

	// Unblock conn.ReadMessage when the context is cancelled.
	go func() {
		<-ctx.Done()
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(2*time.Second),
		)
		_ = conn.Close()
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			return fmt.Errorf("read: %w", err)
		}

		var msg wireMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("certstream: skipping malformed frame: %v", err)
			continue
		}
		if msg.MessageType != "certificate_update" {
			continue
		}

		select {
		case out <- toEntry(msg.Data):
		case <-ctx.Done():
			return nil
		}
	}
}

func toEntry(d wireData) Entry {
	sec := int64(d.Seen)
	nsec := int64((d.Seen - float64(sec)) * 1e9)
	return Entry{
		CertIndex:  d.CertIndex,
		Seen:       time.Unix(sec, nsec).UTC(),
		AllDomains: d.LeafCert.AllDomains,
		LeafCert: LeafCert{
			Subject:     Subject(d.LeafCert.Subject),
			Issuer:      Subject(d.LeafCert.Issuer),
			NotBefore:   time.Unix(int64(d.LeafCert.NotBefore), 0).UTC().Format(time.RFC3339),
			NotAfter:    time.Unix(int64(d.LeafCert.NotAfter), 0).UTC().Format(time.RFC3339),
			Fingerprint: d.LeafCert.Fingerprint,
		},
	}
}

type wireMessage struct {
	MessageType string   `json:"message_type"`
	Data        wireData `json:"data"`
}

type wireData struct {
	CertIndex int64        `json:"cert_index"`
	Seen      float64      `json:"seen"`
	LeafCert  wireLeafCert `json:"leaf_cert"`
}

type wireLeafCert struct {
	Subject     wireSubject `json:"subject"`
	Issuer      wireSubject `json:"issuer"`
	AllDomains  []string    `json:"all_domains"`
	NotBefore   float64     `json:"not_before"`
	NotAfter    float64     `json:"not_after"`
	Fingerprint string      `json:"fingerprint"`
}

type wireSubject struct {
	CN string `json:"CN"`
	O  string `json:"O"`
	C  string `json:"C"`
}
