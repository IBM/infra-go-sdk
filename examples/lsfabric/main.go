package main

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	svc "github.com/sudeeshjohn/svc-go-sdk"
)

func main() {
	// Command line flags
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "REDACTED_SVC_IP<==", "SVC IP address")
	svcUser := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC username")
	svcPass := flag.String("svc-pass", "REDACTED_SVC_PASS<==", "SVC password")
	flag.Parse()
	ctx := context.Background()
	// Initialize the client
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	// Default example output should be visible even without -verbose.
	if *verbose {
		client = client.WithDebug()
		client.Logger.Debug("Verbose mode enabled. Connecting to SVC.", "ip", *svcIP, "user", *svcUser)
	} else {
		client = client.WithLogger(svc.NewLogger(log.InfoLevel, os.Stderr))
	}

	if err := client.Authenticate(ctx); err != nil {
		client.Logger.Error("Authentication error", "error", err)
		os.Exit(1) // Replaces log.Fatalf
	}
	client.Logger.Info("✅ Authenticated")

	// List all fabric logins
	client.Logger.Debug("Fetching fabric logins (this may take a few minutes)...")

	logins, err := client.Lsfabric(ctx)
	if err != nil {
		client.Logger.Error("Lsfabric error", "error", err)
		os.Exit(1)
	}

	if len(logins) == 0 {
		client.Logger.Info("No fabric logins found")
	} else {
		client.Logger.Info("Fabric logins retrieved", "count", len(logins))
		
		// We use Debug here so it only prints the massive list if the user asked for verbose output
		for i, login := range logins {
			client.Logger.Debug("Fabric Login", 
				"index", i+1, 
				"remote_wwpn", login.RemoteWWPN, 
				"host_name", login.HostName, 
				"local_wwpn", login.LocalWWPN, 
				"status", login.State,
			)
		}
	}

	// Define the host parameters
	host := svc.Host{
		Name:     "host1",
		Fcwwpn:   []string{"100000620B42EB0A", "100000620B42EB09"},
		Type:     "generic",
		Protocol: "scsi",
	}

	client.Logger.Info("Checking WWPNs for existing host associations...")
	client.Logger.Debug("WWPN targets", "wwpns", host.Fcwwpn)

	// Check if any WWPN is already associated with a host
	existingHost, matchedWWPN, err := client.GetHostByWWPN(ctx,host.Fcwwpn)
	
	if err == nil && existingHost != "" {
		client.Logger.Info("✅ WWPN is already associated with host", "matched_wwpn", matchedWWPN, "host", existingHost)
		return // Exit with success if already exists
	} else if !strings.Contains(err.Error(), "not found") {
		client.Logger.Error("GetHostByWWPN error", "error", err)
		os.Exit(1)
	}

	client.Logger.Debug("None of the specified WWPNs were found in the fabric")
	client.Logger.Info("✅ No existing host associations found for the specified WWPNs")
}