// Package exutil provides shared utilities for phmc example programs.
// It is intentionally kept minimal: only things every example needs.
package exutil

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// sensitiveHeaders lists header names (lower-cased) whose values are
// replaced with "[REDACTED]" in debug output.
var sensitiveHeaders = map[string]bool{
	"authorization":   true,
	"x-api-session":   true,
	"x-auth-token":    true,
	"x-auth-password": true,
}

// DebugTransport wraps any http.RoundTripper and emits structured
// request/response logs to the standard logger.
//
//   - debug=false, debugFull=false  → one summary line per round-trip
//   - debug=true                    → full headers + body (response bodies
//     truncated at 2 048 bytes)
//   - debugFull=true                → full headers + complete body (no truncation)
type DebugTransport struct {
	Inner     http.RoundTripper
	Debug     bool
	DebugFull bool
}

// RoundTrip implements http.RoundTripper.
func (t *DebugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqID := fmt.Sprintf("%d", time.Now().UnixNano())

	if t.Debug || t.DebugFull {
		t.logRequest(reqID, req)
	}

	start := time.Now()
	resp, err := t.Inner.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("[http] %-6s %s  =>  ERROR: %v  (%s)", req.Method, req.URL, err, elapsed)
		return nil, err
	}

	if t.Debug || t.DebugFull {
		t.logResponse(reqID, resp, elapsed)
	} else {
		log.Printf("[http] %-6s %s  =>  %d  (%s)", req.Method, req.URL, resp.StatusCode, elapsed)
	}

	return resp, nil
}

func (t *DebugTransport) logRequest(reqID string, req *http.Request) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n┌─ REQUEST  [%s] ────────────────────────────────────────\n", reqID)
	fmt.Fprintf(&sb, "│  %s %s %s\n", req.Method, req.URL, req.Proto)
	fmt.Fprintf(&sb, "│  Host: %s\n", req.Host)
	for k, vv := range req.Header {
		lk := strings.ToLower(k)
		val := strings.Join(vv, ", ")
		if sensitiveHeaders[lk] {
			val = "[REDACTED]"
		}
		fmt.Fprintf(&sb, "│  %s: %s\n", k, val)
	}
	if req.Body != nil && req.Body != http.NoBody {
		body, err := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body))
		if err == nil && len(body) > 0 {
			fmt.Fprintf(&sb, "│  Body (%d bytes):\n", len(body))
			for _, line := range strings.Split(string(body), "\n") {
				fmt.Fprintf(&sb, "│    %s\n", line)
			}
		}
	} else {
		fmt.Fprintf(&sb, "│  Body: (empty)\n")
	}
	fmt.Fprintf(&sb, "└────────────────────────────────────────────────────────")
	log.Print(sb.String())
}

func (t *DebugTransport) logResponse(reqID string, resp *http.Response, elapsed time.Duration) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n┌─ RESPONSE [%s] ────────────────────────────────────────\n", reqID)
	fmt.Fprintf(&sb, "│  %s  (%s)\n", resp.Status, elapsed)
	for k, vv := range resp.Header {
		fmt.Fprintf(&sb, "│  %s: %s\n", k, strings.Join(vv, ", "))
	}
	if resp.Body != nil {
		body, err := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewReader(body))
		if err == nil && len(body) > 0 {
			const maxPreview = 2048
			if !t.DebugFull && len(body) > maxPreview {
				fmt.Fprintf(&sb, "│  Body (%d bytes):\n", len(body))
				for _, line := range strings.Split(string(body[:maxPreview]), "\n") {
					fmt.Fprintf(&sb, "│    %s\n", line)
				}
				fmt.Fprintf(&sb, "│    … [truncated at %d bytes; use --debug-full for complete body]\n", maxPreview)
			} else {
				fmt.Fprintf(&sb, "│  Body (%d bytes):\n", len(body))
				for _, line := range strings.Split(string(body), "\n") {
					fmt.Fprintf(&sb, "│    %s\n", line)
				}
			}
		} else {
			fmt.Fprintf(&sb, "│  Body: (empty)\n")
		}
	}
	fmt.Fprintf(&sb, "└────────────────────────────────────────────────────────")
	log.Print(sb.String())
}
