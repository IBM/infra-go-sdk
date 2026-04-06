package main

import (
	"fmt"
	"log"

	"github.com/sudeeshjohn/svc-go-sdk" // Adjust if your package path differs
)

func main() {
	// Initialize Client
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// Fetch all FC ports
	ports, err := client.Lsportfc()
	if err != nil {
		log.Fatalf("Lsportfc error: %v", err)
	}
	
	if len(ports) == 0 {
		fmt.Println("No FC ports found.")
		return
	}

	fmt.Printf("Total FC ports found: %d\n\n", len(ports))
	
	// Print active ports specifically useful for zoning
	fmt.Println("--- Active FC Ports ---")
	for _, port := range ports {
		if port.Status == "active" {
			fmt.Printf("Node: %s | Adapter: %s | Port ID: %s | WWPN: %s | Speed: %s\n", 
				port.NodeName, port.AdapterLocation, port.PortID, port.WWPN, port.PortSpeed)
		}
	}
}