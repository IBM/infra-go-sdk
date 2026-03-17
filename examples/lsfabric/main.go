package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	svc "github.com/sudeeshjohn/svc-go-sdk"
)

func main() {
	// Command line flags
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "REDACTED_SVC_IP<==", "SVC IP address")
	svcUser := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC username")
	svcPass := flag.String("svc-pass", "REDACTED_SVC_PASS<==", "SVC password")
	flag.Parse()

	if *verbose {
		log.Printf("Connecting to SVC at %s as user %s", *svcIP, *svcUser)
	}

	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// List all fabric logins
	if *verbose {
		log.Println("Fetching fabric logins (this may take a few minutes)...")
	}

	logins, err := client.Lsfabric()
	if err != nil {
		log.Fatalf("Lsfabric error: %v", err)
	}

	if len(logins) == 0 {
		fmt.Println("No fabric logins found")
	} else {
		fmt.Printf("Found %d fabric login(s)\n", len(logins))
		if *verbose {
			fmt.Println("\nAll Fabric Logins:")
			for i, login := range logins {
				fmt.Printf("%d. Remote WWPN: %s, Host Name: %s, Local WWPN: %s, Status: %s\n",
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

	fmt.Println("\nChecking WWPNs for existing host associations...")
	// Check if any WWPN is already associated with a host
	if *verbose {
		log.Printf("Checking WWPNs: %v", host.Fcwwpn)
	}

	existingHost, matchedWWPN, err := client.GetHostByWWPN(host.Fcwwpn)
	if err == nil && existingHost != "" {
		fmt.Printf("✅ WWPN %s is already associated with host: %s\n", matchedWWPN, existingHost)
		return // Exit with success if already exists
	} else if !strings.Contains(err.Error(), "not found") {
		log.Fatalf("GetHostByWWPN error: %v", err)
	}

	if *verbose {
		log.Printf("None of the WWPNs found in fabric")
	}
	fmt.Println("✅ No existing host associations found for the specified WWPNs")
}
