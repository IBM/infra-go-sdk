package main

import (
	"log"
	"context"
	"flag"
	"os"

	"github.com/IBM/infra-go-sdk/svc" // Adjust if your package path differs
)

func main() {
	// Command line flags
	verbose := flag.Bool("verbose", false, "Enable verbose output to see inactive ports")
	svcIP := flag.String("svc-ip", "", "SVC IP address (required)")
	svcUser := flag.String("svc-user", "", "SVC username (required)")
	svcPass := flag.String("svc-pass", "", "SVC password (required)")
	flag.Parse()

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: lsportfc -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()
	// Initialize the client
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	// Enable debug logging if the verbose flag is passed
	if *verbose {
		log.Printf("Verbose mode enabled. Connecting to SVC.: ip=%v user=%v", *svcIP, *svcUser)
	}

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}
	log.Println("✅ Authenticated")

	// Fetch all FC ports
	log.Println("Fetching all FC ports...")
	ports, err := client.Lsportfc(ctx)
	if err != nil {
		log.Printf("Lsportfc error: error=%v", err)
		os.Exit(1)
	}

	if len(ports) == 0 {
		log.Println("No FC ports found on the system.")
		return
	}

	log.Printf("✅ FC ports retrieved: total_ports=%v", len(ports))

	activeCount := 0
	for _, port := range ports {
		if port.Status == "active" {
			activeCount++
			// Log active ports at INFO level so they are always visible for zoning
			log.Printf("[INFO] Active FC Port %v", "node", port.NodeName,
				"adapter", port.AdapterLocation,
				"port_id", port.PortID,
				"wwpn", port.WWPN,
				"speed", port.PortSpeed,)
		} else {
			// Log inactive/unconfigured ports at DEBUG level
			log.Printf("[DEBUG] Inactive/Other FC Port %v", "node", port.NodeName,
				"port_id", port.PortID,
				"wwpn", port.WWPN,
				"status", port.Status,)
		}
	}

	log.Printf("Port summary: active_ports=%v total_ports=%v", activeCount, len(ports))
}
