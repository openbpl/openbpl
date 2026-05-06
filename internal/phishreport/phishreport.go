package phishreport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var client = &http.Client{Timeout: 10 * time.Second}

// Contact holds abuse contact info returned by the phish.report API.
type Contact struct {
	Name      string `json:"name"`
	ReportURI string `json:"report_uri"`
	Role      string `json:"role"`
}

// Lookup calls the phish.report hosting API to find registrar and hosting
// abuse contacts for the given domain. No API key required.
func Lookup(domain string) ([]Contact, error) {
	u := "https://phish.report/api/v0/hosting?url=" + url.QueryEscape("https://"+domain)
	resp, err := client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("phishreport lookup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("phishreport: %d %s", resp.StatusCode, string(body))
	}

	var contacts []Contact
	if err := json.NewDecoder(resp.Body).Decode(&contacts); err != nil {
		return nil, fmt.Errorf("phishreport decode: %w", err)
	}
	return contacts, nil
}
