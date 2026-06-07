package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc"
)

func main() {
	// Define command-line flags
	hmcIP := flag.String("hmc-ip", "", "HMC IP address (required)")
	username := flag.String("username", "", "HMC username (required)")
	password := flag.String("password", "", "HMC password (required)")
	systemName := flag.String("system-name", "", "Managed system name (required)")
	insecure := flag.Bool("insecure", false, "Skip SSL certificate verification")
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Get virtual switch information from an HMC-managed system.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -hmc-ip 192.0.2.1 -username REDACTED_HMC_USER<== -password mypass -system-name mysystem\n", os.Args[0])
	}
	
	flag.Parse()

	// Validate required flags
	if *hmcIP == "" || *username == "" || *password == "" || *systemName == "" {
		fmt.Fprintf(os.Stderr, "Error: All flags are required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Create HMC client
	client := hmc.NewRestClient(*hmcIP)
	
	// Login to HMC
	if err := client.Login(context.Background(), *username, *password, *insecure); err != nil {
		log.Fatalf("Failed to login to HMC: %v", err)
	}
	defer client.Logoff(context.Background())

	// Get system UUID from system name
	systems, err := client.GetManagedSystemQuickAll(context.Background(), false)
	if err != nil {
		log.Fatalf("Failed to get managed systems: %v", err)
	}

	var sysUUID string
	for _, sys := range systems {
		if sys.SystemName == *systemName {
			sysUUID = sys.UUID
			fmt.Printf("Found system '%s' with UUID: %s\n\n", *systemName, sysUUID)
			break
		}
	}

	if sysUUID == "" {
		log.Fatalf("System '%s' not found on HMC", *systemName)
	}

	// 1. Get all virtual switches (Quick method)
	fmt.Println("=== Virtual Switches (Quick All) ===")
	quickAll, err := client.GetVirtualSwitchQuickAll(context.Background(), sysUUID, *insecure)
	if err != nil {
		log.Fatalf("Failed to get virtual switches: %v", err)
	}
	
	if len(quickAll) == 0 {
		fmt.Println("No virtual switches found on this system")
		return
	}
	
	for _, s := range quickAll {
		fmt.Printf("Name: %s | UUID: %s\n", s.SwitchName, s.UUID)
	}

	// 2. Get detailed information for first switch (Quick Singular)
	fmt.Println("\n=== Virtual Switch Details (Quick Singular) ===")
	if len(quickAll) > 0 {
		quickOne, err := client.GetVirtualSwitchQuick(context.Background(), sysUUID, quickAll[0].UUID, *insecure)
		if err != nil {
			log.Printf("Warning: Failed to get switch details: %v", err)
		} else {
			fmt.Printf("Switch Name: %s\n", quickOne.SwitchName)
			fmt.Printf("Switch Mode: %s\n", quickOne.SwitchMode)
			fmt.Printf("Switch UUID: %s\n", quickOne.UUID)
		}
	}

	// 3. Get comprehensive XML feed
	fmt.Println("\n=== Virtual Switches (Comprehensive XML) ===")
	xmlSwitches, err := client.GetVirtualSwitches(context.Background(), sysUUID, *insecure)
	if err != nil {
		log.Printf("Warning: Failed to get comprehensive switch data: %v", err)
	} else {
		for _, s := range xmlSwitches {
			fmt.Printf("\nSwitch: %s (ID: %s)\n", s.SwitchName, s.SwitchID)
			if len(s.VirtualNetworks) > 0 {
				fmt.Println("  Attached Virtual Networks:")
				for i, net := range s.VirtualNetworks {
					fmt.Printf("    %d. %s\n", i+1, net)
				}
			} else {
				fmt.Println("  No attached virtual networks")
			}
		}
	}
	
	fmt.Println("\n✓ Successfully retrieved virtual switch information")
}
