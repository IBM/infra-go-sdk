package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

// debugTransport wraps any http.RoundTripper.
// When debug is true it prints full request/response detail (bodies truncated to 2048 bytes).
// When debugFull is true it prints full request/response without truncation.
// When both are false it prints a single summary line per round-trip.
type debugTransport struct {
	inner     http.RoundTripper
	debug     bool
	debugFull bool
}

var sensitiveHeaders = map[string]bool{
	"authorization":   true,
	"x-api-session":   true,
	"x-auth-token":    true,
	"x-auth-password": true,
}

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

func (t *debugTransport) logResponse(reqID string, resp *http.Response, elapsed time.Duration) {
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
			if !t.debugFull {
				const maxPreview = 2048
				preview := body
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
					fmt.Fprintf(&sb, "│    … [truncated at %d bytes; use --debug-full for the full body]\n", maxPreview)
				}
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

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP     := flag.String("hmc-ip",     "", "HMC IP address")
	username  := flag.String("hmc-user",   "", "HMC username")
	password  := flag.String("hmc-pass",   "", "HMC password")
	verbose   := flag.Bool("verbose",   false, "Enable verbose output")
	debug     := flag.Bool("debug",     false, "Enable request/response dump with truncated large bodies")
	debugFull := flag.Bool("debug-full", false, "Enable full request/response dump including complete response bodies")

	flag.Parse()

	if *password == "" {
		log.Fatal("❌ Error: hmc-pass is required.")
	}

	if *verbose {
		log.Printf("Connecting to HMC %s as %s", *hmcIP, *username)
	}

	// =========================================================================
	// BUILD CLIENT WITH OPTIONAL HTTP TRANSPORT WRAPPER
	// =========================================================================
	// 1. NewRestClient   — plain http.Client, standard TLS verification.
	// 2. WithTLSInsecure — clones http.DefaultTransport, sets InsecureSkipVerify.
	// 3. HTTPTransport   — retrieves the patched transport so we can wrap it.
	// 4. WithTransport   — installs debugTransport around the inner transport.
	//
	// Order matters: WithTLSInsecure must come *before* WithTransport so the
	// debugTransport's inner field already carries InsecureSkipVerify.
	restClient := hmc.NewRestClient(*hmcIP).WithTLSInsecure()
	baseTransport := restClient.HTTPTransport()
	restClient.WithTransport(&debugTransport{
		inner:     baseTransport,
		debug:     *debug || *debugFull,
		debugFull: *debugFull,
	})

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	if *verbose {
		log.Println("Authenticating to HMC...")
	}
	if err := restClient.Login(context.Background(), *username, *password); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	if *verbose {
		log.Println("Authentication successful")
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// FETCH ALL LPARS ACROSS THE ENTIRE HMC
	// =========================================================================
	fmt.Printf("\n🌍 Querying HMC for ALL Managed Logical Partitions...\n")
	if *verbose {
		log.Println("Fetching all logical partitions across the HMC...")
	}

	partitions, err := restClient.GetAllLogicalPartitionsInHmc()
	if err != nil {
		log.Fatalf("❌ Failed to retrieve partitions: %v", err)
	}
	if *verbose {
		log.Printf("Retrieved %d partitions", len(partitions))
	}

	if len(partitions) == 0 {
		fmt.Println("No partitions found on this HMC.")
		return
	}

	// =========================================================================
	// DISPLAY RESULTS IN A TABLE
	// =========================================================================
	fmt.Printf("\n✅ Found %d Partitions globally across the HMC:\n", len(partitions))
	fmt.Println("========================================================================================================================")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SYSTEM NAME\tLPAR NAME\tID\tSTATE\tTYPE\tUUID")
	fmt.Fprintln(w, "-----------\t---------\t--\t-----\t----\t----")

	for _, lpar := range partitions {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
			lpar.SystemName,
			lpar.PartitionName,
			lpar.PartitionID,
			lpar.PartitionState,
			lpar.PartitionType,
			lpar.PartitionUUID,
		)
	}

	w.Flush()
	fmt.Println("========================================================================================================================")
}
