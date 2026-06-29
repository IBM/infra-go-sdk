package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	svc "github.com/IBM/infra-go-sdk/svc"
)

func main() {
	// Command line flags
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "", "SVC IP address (required)")
	svcUser := flag.String("svc-user", "", "SVC username (required)")
	svcPass := flag.String("svc-pass", "", "SVC password (required)")
	flag.Parse()

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: lsfabric -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(ctx); err != nil {
		log.Fatalf("Authentication error: %v", err)
	}
	log.Println("Authenticated successfully")

	log.Println("Fetching fabric logins (this may take a few minutes)...")

	logins, err := client.Lsfabric(ctx)
	if err != nil {
		log.Fatalf("Lsfabric error: %v", err)
	}

	if len(logins) == 0 {
		log.Println("No fabric logins found")
	} else {
		log.Printf("Fabric logins retrieved: count=%d", len(logins))
		if *verbose {
			for i, login := range logins {
				log.Printf("  [%d] remote_wwpn=%s host_name=%s local_wwpn=%s status=%s",
					i+1, login.RemoteWWPN, login.HostName, login.LocalWWPN, login.State)
			}
		}
	}

	// Define the host parameters
	host := svc.Host{
		Name:     "host1",
		Fcwwpn:   []string{"100000620B42EB0A", "100000620B42EB09"},
		Type:     "generic",
		Protocol: "scsi",
	}

	log.Println("Checking WWPNs for existing host associations...")

	existingHost, matchedWWPN, err := client.GetHostByWWPN(ctx, host.Fcwwpn)

	if err == nil && existingHost != "" {
		log.Printf("WWPN is already associated with host: matched_wwpn=%s host=%s", matchedWWPN, existingHost)
		return
	} else if err != nil && !strings.Contains(err.Error(), "not found") {
		log.Fatalf("GetHostByWWPN error: %v", err)
		os.Exit(1)
	}

	log.Println("No existing host associations found for the specified WWPNs")
}
