package main

import (
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
	logger := svc.NewDefaultLogger()

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		logger.Fatal("Usage: lsportfc -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()
	// Initialize the client
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	// Enable debug logging if the verbose flag is passed
	if *verbose {
		client = client.WithDebug()
		client.Logger.Debug("Verbose mode enabled. Connecting to SVC.", "ip", *svcIP, "user", *svcUser)
	}

	if err := client.Authenticate(ctx); err != nil {
		client.Logger.Error("Authentication error", "error", err)
		os.Exit(1)
	}
	client.Logger.Info("✅ Authenticated")

	// Fetch all FC ports
	client.Logger.Info("Fetching all FC ports...")
	ports, err := client.Lsportfc(ctx)
	if err != nil {
		client.Logger.Error("Lsportfc error", "error", err)
		os.Exit(1)
	}

	if len(ports) == 0 {
		client.Logger.Info("No FC ports found on the system.")
		return
	}

	client.Logger.Info("✅ FC ports retrieved", "total_ports", len(ports))

	activeCount := 0
	for _, port := range ports {
		if port.Status == "active" {
			activeCount++
			// Log active ports at INFO level so they are always visible for zoning
			client.Logger.Info("Active FC Port",
				"node", port.NodeName,
				"adapter", port.AdapterLocation,
				"port_id", port.PortID,
				"wwpn", port.WWPN,
				"speed", port.PortSpeed,
			)
		} else {
			// Log inactive/unconfigured ports at DEBUG level
			client.Logger.Debug("Inactive/Other FC Port",
				"node", port.NodeName,
				"port_id", port.PortID,
				"wwpn", port.WWPN,
				"status", port.Status,
			)
		}
	}

	client.Logger.Info("Port summary", "active_ports", activeCount, "total_ports", len(ports))
}