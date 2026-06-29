// withtransport demonstrates how to inject a custom http.RoundTripper into the
// HMC (phmc) SDK client.
//
// Run without -debug for a concise one-line-per-request summary.
// Run with -debug for full request/response dump (headers + body).
//
// Usage:
//
//	go run ./phmc/examples/withtransport \
//	    -hmc-ip 192.0.2.1 \
//	    -hmc-user hscroot \
//	    -hmc-pass secret \
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

	hmc "github.com/IBM/infra-go-sdk/phmc"
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
	"authorization":   true,
	"x-api-session":   true,
	"x-auth-token":    true,
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

	// Body snapshot (read + restore so the SDK can still decode the response)
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
	hmcIP   := flag.String("hmc-ip",   "", "HMC IP address  (required)")
	hmcUser := flag.String("hmc-user", "", "HMC username    (required)")
	hmcPass := flag.String("hmc-pass", "", "HMC password    (required)")
	debug   := flag.Bool("debug",      false, "Enable full request/response dump")
	flag.Parse()

	if *hmcIP == "" || *hmcUser == "" || *hmcPass == "" {
		log.Fatal("Usage: withtransport -hmc-ip <ip> -hmc-user <user> -hmc-pass <pass> [-debug]")
	}

	// ── Build the client ──────────────────────────────────────────────────────
	//
	// 1. NewRestClient   — plain http.Client, standard TLS verification.
	// 2. WithTLSInsecure — clones http.DefaultTransport, sets InsecureSkipVerify.
	// 3. HTTPTransport   — retrieves the patched transport so we can wrap it.
	// 4. WithTransport   — installs the debugTransport around the inner transport.
	//
	// Order matters: WithTLSInsecure must come *before* WithTransport so the
	// debugTransport's inner field already carries InsecureSkipVerify.

	restClient := hmc.NewRestClient(*hmcIP).WithTLSInsecure()

	restClient.WithTransport(&debugTransport{
		inner: restClient.HTTPTransport(),
		debug: *debug,
	})

	// ─────────────────────────────────────────────────────────────────────────

	ctx := context.Background()

	log.Println("Logging in…")
	if err := restClient.Login(ctx, *hmcUser, *hmcPass, false); err != nil {
		log.Fatalf("login failed: %v", err)
	}
	log.Println("Logged in successfully")

	defer func() {
		log.Println("Logging off…")
		if err := restClient.Logoff(ctx); err != nil {
			log.Printf("logoff warning: %v", err)
		}
	}()

	log.Println("Fetching managed systems…")
	systems, err := restClient.GetManagedSystemQuickAll(ctx, false)
	if err != nil {
		log.Fatalf("GetManagedSystemQuickAll failed: %v", err)
	}

	log.Printf("Found %d managed system(s):", len(systems))
	for _, s := range systems {
		log.Printf("  name=%-30s  uuid=%s  state=%s", s.SystemName, s.UUID, s.State)
	}
}
