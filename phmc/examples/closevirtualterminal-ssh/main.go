package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	hmc "github.com/IBM/infra-go-sdk/phmc"
)

func main() {
	// Define command-line flags
	hmcIP := flag.String("hmc", "", "HMC IP address (required)")
	username := flag.String("user", "", "HMC username (required)")
	password := flag.String("pass", "", "HMC password (required)")
	systemName := flag.String("system", "", "Managed system name (required)")
	lparName := flag.String("lpar", "", "LPAR name (required)")
	verbose := flag.Bool("verbose", true, "Enable verbose output")
	useSSH := flag.Bool("ssh", true, "Use SSH instead of REST API (bypasses CLIRunner limitations)")

	flag.Parse()
	_ = verbose

	// Validate required flags
	if *hmcIP == "" || *username == "" || *password == "" || *systemName == "" || *lparName == "" {
		fmt.Println("Error: All flags are required")
		fmt.Println("\nUsage:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  # Using REST API CLIRunner:")
		fmt.Println("  go run main.go -hmc 192.0.2.2 -user REDACTED_HMC_USER<== -pass <password> -system LTC09u23-p11 -lpar sno-new-4 -verbose")
		fmt.Println()
		fmt.Println("  # Using direct SSH (bypasses REST API limitations):")
		fmt.Println("  go run main-ssh.go -hmc 192.0.2.2 -user REDACTED_HMC_USER<== -pass <password> -system LTC09u23-p11 -lpar sno-new-4 -ssh -verbose")
		os.Exit(1)
	}

	if *useSSH {
		// Use SSH method (bypasses REST API)
		fmt.Println("Using SSH method to close virtual terminal...")
		fmt.Printf("Connecting to HMC at %s via SSH...\n", *hmcIP)

		// Create HMC client (needed for the method)
		client := hmc.NewRestClient(*hmcIP)

		// Close virtual terminal via SSH
		fmt.Printf("\nClosing virtual terminal for LPAR '%s' on system '%s' via SSH...\n", *lparName, *systemName)
		if err := client.CloseVirtualTerminalViaSSH(*hmcIP, *username, *password, *systemName, *lparName, *verbose); err != nil {
			log.Fatalf("Failed to close virtual terminal via SSH: %v", err)
		}

		fmt.Println("✓ Virtual terminal closed successfully via SSH")
	} else {
		// Use REST API method
		fmt.Println("Using REST API CLIRunner to close virtual terminal...")
		fmt.Printf("Connecting to HMC at %s...\n", *hmcIP)
		client := hmc.NewRestClient(*hmcIP)

		// Login to HMC
		if err := client.Login(context.Background(), *username, *password, *verbose); err != nil {
			log.Fatalf("Failed to login to HMC: %v", err)
		}
		fmt.Println("✓ Successfully logged in to HMC")

		// Close virtual terminal
		fmt.Printf("\nClosing virtual terminal for LPAR '%s' on system '%s' via REST API...\n", *lparName, *systemName)
		if err := client.CloseVirtualTerminal(context.Background(), *systemName, *lparName, *verbose); err != nil {
			log.Fatalf("Failed to close virtual terminal: %v", err)
		}

		fmt.Println("✓ Virtual terminal closed successfully via REST API")
	}
}

// Made with Bob
