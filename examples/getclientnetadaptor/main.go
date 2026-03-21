package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	lparName := flag.String("lpar-name", "Go_LPAR_91", "Target LPAR Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" {
		log.Fatal("❌ Error: hmc-pass is required.")
	}

	// 1. Initialize & Login
	client := hmc.NewHmcRestClient(*hmcIP)
	if err := client.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer client.Logoff()

	// 2. Resolve System UUID dynamically
	_, sysUUID, err := client.GetManagedSystemByNameQuick(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ Managed System '%s' not found: %v", *sysName, err)
	}

	// 3. Resolve LPAR UUID and CHECK IF PRESENT 
	if *verbose {
		fmt.Printf("Searching for LPAR '%s' on system '%s'...\n", *lparName, *sysName)
	}
	lparDetails, lparUUID, err := client.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	
	// EXISTENCE CHECK
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ ABORT: LPAR '%s' is not present on system '%s'. Error: %v", *lparName, *sysName, err)
	}

	if *verbose {
		fmt.Printf("✅ LPAR found! UUID: %s | Current State: %s\n", lparUUID, lparDetails.PartitionState)
	}

	// 4. Fetch Client Network Adapters
	adapters, err := client.GetClientNetworkAdapters(sysUUID, lparUUID, *verbose)
	if err != nil {
		log.Fatalf("Error fetching adapters: %v", err)
	}

	if len(adapters) == 0 {
		fmt.Printf("No Client Network Adapters found for LPAR '%s'.\n", *lparName)
		return
	}

	// 5. Display Table of Adapters
	fmt.Printf("\nNetwork Adapters for %s:\n", *lparName)
	for i, a := range adapters {
		fmt.Printf("\n--- Adapter %d ---\n", i+1)
		fmt.Printf("UUID:           %s\n", a.UUID)
		fmt.Printf("MAC Address:    %s\n", a.MACAddress)
		fmt.Printf("Port VLAN ID:   %s\n", a.PortVLANID)
		fmt.Printf("Virtual Switch: %s (ID: %s)\n", a.VirtualSwitchName, a.VirtualSwitchID)
		fmt.Printf("Switch URI:     %s\n", a.AssociatedVirtualSwitchURI.Href)
		
		fmt.Println("Attached Networks:")
		for _, net := range a.VirtualNetworkURIs {
			fmt.Printf("  - %s\n", net.Href)
		}
		fmt.Println(strings.Repeat("-", 60))
	}
}