// withtransport demonstrates how to inject a custom http.RoundTripper into the
// SVC SDK client.
//
// Run without -debug for a concise one-line-per-request summary.
// Run with -debug for full request/response dump (headers + body).
//
// Usage:
//
//	go run ./svc/examples/withtransport \
//	    -svc-ip 192.0.2.10 \
//	    -svc-user admin \
//	    -svc-pass secret \
//	    [-debug]
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	svc "github.com/IBM/infra-go-sdk/svc"
)

// ── transport ────────────────────────────────────────────────────────────────

// debugTransport wraps any http.RoundTripper.
// When debug is true it prints full request/response detail.
// When debug is false it prints a single summary line per round-trip.
type debugTransport struct {
	inner http.RoundTripper
	debug bool
}

// sensitiveHeaders are redacted in debug output to avoid leaking credentials.
var sensitiveHeaders = map[string]bool{
	"authorization":  true,
	"x-auth-token":   true,
	"x-auth-password": true,
}

// RoundTrip implements http.RoundTripper.
func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqID := fmt.Sprintf("%d", time.Now().UnixNano())

	if t.debug {
		t.logRequest(reqID, req)
	}

	start := time.Now()
	resp, err := t.inner.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("[http] %-6s %s  =>  ERROR: %v  (%s)", req.Method, req.URL, err, elapsed)
		return nil, err
	}

	if t.debug {
		t.logResponse(reqID, resp, elapsed)
	} else {
		log.Printf("[http] %-6s %s  =>  %d  (%s)", req.Method, req.URL, resp.StatusCode, elapsed)
	}

	return resp, nil
}

func (t *debugTransport) logRequest(reqID string, req *http.Request) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n┌─ REQUEST  [%s] ────────────────────────────────────────\n", reqID)
	fmt.Fprintf(&sb, "│  %s %s %s\n", req.Method, req.URL, req.Proto)
	fmt.Fprintf(&sb, "│  Host: %s\n", req.Host)

	// Headers — redact sensitive values
	for k, vv := range req.Header {
		lk := strings.ToLower(k)
		val := strings.Join(vv, ", ")
		if sensitiveHeaders[lk] {
			val = "[REDACTED]"
		}
		fmt.Fprintf(&sb, "│  %s: %s\n", k, val)
	}

	// Body snapshot (read + restore)
	if req.Body != nil && req.Body != http.NoBody {
		body, err := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body)) // restore for the actual send
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

func (t *debugTransport) logResponse(reqID string, resp *http.Response, elapsed time.Duration) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n┌─ RESPONSE [%s] ────────────────────────────────────────\n", reqID)
	fmt.Fprintf(&sb, "│  %s  (%s)\n", resp.Status, elapsed)

	for k, vv := range resp.Header {
		fmt.Fprintf(&sb, "│  %s: %s\n", k, strings.Join(vv, ", "))
	}

	// Body snapshot (read + restore so the SDK can still decode it)
	if resp.Body != nil {
		body, err := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewReader(body)) // restore
		if err == nil && len(body) > 0 {
			preview := body
			const maxPreview = 2048
			truncated := false
			if len(preview) > maxPreview {
				preview = preview[:maxPreview]
				truncated = true
			}
			fmt.Fprintf(&sb, "│  Body (%d bytes):\n", len(body))
			for _, line := range strings.Split(string(preview), "\n") {
				fmt.Fprintf(&sb, "│    %s\n", line)
			}
			if truncated {
				fmt.Fprintf(&sb, "│    … [truncated at %d bytes]\n", maxPreview)
			}
		} else {
			fmt.Fprintf(&sb, "│  Body: (empty)\n")
		}
	}

	fmt.Fprintf(&sb, "└────────────────────────────────────────────────────────")
	log.Print(sb.String())
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	svcIP   := flag.String("svc-ip",   "", "SVC IP address  (required)")
	svcUser := flag.String("svc-user", "", "SVC username    (required)")
	svcPass := flag.String("svc-pass", "", "SVC password    (required)")
	debug   := flag.Bool("debug",      false, "Enable full request/response dump")
	flag.Parse()

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: withtransport -svc-ip <ip> -svc-user <user> -svc-pass <pass> [-debug]")
	}

	// ── Build the client ──────────────────────────────────────────────────────
	//
	// 1. NewClient       — plain http.Client, default transport.
	// 2. WithTLSInsecure — clones http.DefaultTransport, sets InsecureSkipVerify.
	// 3. WithTransport   — wraps the TLS-patched transport with our debugTransport.
	//
	// Order matters: WithTLSInsecure must come *before* WithTransport so the
	// debugTransport's inner field already carries InsecureSkipVerify.

	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	client.WithTransport(&debugTransport{
		inner: client.HTTPClient.Transport,
		debug: *debug,
	})

	// ─────────────────────────────────────────────────────────────────────────

	ctx := context.Background()

	log.Println("Authenticating…")
	if err := client.Authenticate(ctx); err != nil {
		log.Fatalf("authentication failed: %v", err)
	}
	log.Println("Authenticated successfully")

	log.Println("Fetching system info…")
	info, err := client.Lssystem(ctx)
	if err != nil {
		log.Fatalf("lssystem failed: %v", err)
	}

	log.Printf("Connected to: name=%s  id=%s  location=%s",
		info.Name, info.ID, info.Location)
}
