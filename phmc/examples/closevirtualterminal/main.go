package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	hmc "github.com/IBM/infra-go-sdk/phmc"
)

func main() {
	// Define command-line flags
	hmcIP := flag.String("hmc", "", "HMC IP address (required)")
	username := flag.String("user", "", "HMC username (required)")
	password := flag.String("pass", "", "HMC password (required)")
	systemName := flag.String("system", "", "Managed system name (required)")
	lparName := flag.String("lpar", "", "LPAR name (required)")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	useFallback := flag.Bool("fallback", true, "Use SSH fallback if REST API fails")

	flag.Parse()
	_ = verbose

	// Validate required flags
	if *hmcIP == "" || *username == "" || *password == "" || *systemName == "" || *lparName == "" {
		fmt.Println("Error: All flags are required")
		fmt.Println("\nUsage:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  # Try REST API first, fallback to SSH if it fails:")
		fmt.Println("  go run main.go -hmc 192.0.2.2 -user REDACTED_HMC_USER<== -pass <password> -system LTC09u23-p11 -lpar sno-new-4 -verbose")
		fmt.Println()
		fmt.Println("  # Use REST API only (no fallback):")
		fmt.Println("  go run main.go -hmc 192.0.2.2 -user REDACTED_HMC_USER<== -pass <password> -system LTC09u23-p11 -lpar sno-new-4 -fallback=false")
		os.Exit(1)
	}

	// Create HMC client
	fmt.Printf("Connecting to HMC at %s...\n", *hmcIP)
	client := hmc.NewRestClient(*hmcIP)

	// Login to HMC
	if err := client.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("Failed to login to HMC: %v", err)
	}
	fmt.Println("✓ Successfully logged in to HMC")

	// Close virtual terminal with fallback support
	fmt.Printf("\nClosing virtual terminal for LPAR '%s' on system '%s'...\n", *lparName, *systemName)
	fmt.Println("Attempting REST API method...")
	
	err := client.CloseVirtualTerminal(context.Background(), *systemName, *lparName, *verbose)
	if err != nil {
		// Check if it's an unsupported command error
		if *useFallback && strings.Contains(err.Error(), "Unsupported command") {
			fmt.Printf("⚠ REST API failed: %v\n", err)
			fmt.Println("Falling back to SSH method...")
			
			// Try SSH method
			if err := client.CloseVirtualTerminalViaSSH(*hmcIP, *username, *password, *systemName, *lparName, *verbose); err != nil {
				log.Fatalf("Failed to close virtual terminal via SSH: %v", err)
			}
			fmt.Println("✓ Virtual terminal closed successfully via SSH")
		} else {
			log.Fatalf("Failed to close virtual terminal: %v", err)
		}
	} else {
		fmt.Println("✓ Virtual terminal closed successfully via REST API")
	}
}

// Made with Bob
