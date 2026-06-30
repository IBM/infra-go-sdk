package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	hmc "github.com/IBM/infra-go-sdk/phmc"
	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

func main() {
	// --- Configuration Flags ---
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	systemName := flag.String("system-name", "", "Managed System Name")
	viosName := flag.String("vios-name", "", "VIOS Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	jsonOutput := flag.Bool("json", false, "Output in JSON format")

	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose

	// Validate required parameters
	if *hmcIP == "" || *username == "" || *password == "" || *systemName == "" || *viosName == "" {
		log.Fatal("Error: hmc-ip, hmc-user, hmc-pass, system-name, and vios-name are required.")
	}

	// --- Initialize and Login ---
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)
	if err := restClient.Login(context.Background(), *username, *password); err != nil {
		log.Fatalf("❌ HMC Login failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	fmt.Println("✅ Successfully authenticated with HMC.")

	// --- Resolve Managed System UUID ---
	fmt.Printf("\nResolving UUID for Managed System '%s'...\n", *systemName)
	_, systemUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *systemName)
	if err != nil || systemUUID == "" {
		log.Fatalf("❌ Managed System '%s' not found: %v", *systemName, err)
	}
	fmt.Printf("✅ Found Managed System UUID: %s\n", systemUUID)

	// --- Resolve VIOS UUID ---
	fmt.Printf("\nResolving UUID for VIOS '%s'...\n", *viosName)
	viosUUID, err := hmc.GetViosID(context.Background(), restClient, systemUUID, *viosName)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found: %v", *viosName, err)
	}
	fmt.Printf("✅ Found VIOS UUID: %s\n", viosUUID)

	// --- Fetch Physical Fibre Channel Ports ---
	fmt.Printf("\n🔍 Fetching Physical Fibre Channel Ports for VIOS '%s'...\n", *viosName)
	fcPorts, err := restClient.GetPhysicalFibreChannelPorts(context.Background(), viosUUID)
	if err != nil {
		log.Fatalf("❌ Failed to fetch Physical FC Ports: %v", err)
	}

	// --- Display Results ---
	if len(fcPorts) == 0 {
		fmt.Println("\n⚠️  No Physical Fibre Channel Ports found on this VIOS.")
		return
	}

	fmt.Printf("\n✅ Found %d Physical Fibre Channel Port(s):\n", len(fcPorts))
	fmt.Println("=========================================================================")

	if *jsonOutput {
		// Output in JSON format
		output, err := json.MarshalIndent(fcPorts, "", "  ")
		if err != nil {
			log.Fatalf("❌ Failed to marshal FC ports to JSON: %v", err)
		}
		fmt.Println(string(output))
	} else {
		// Output in human-readable table format
		fmt.Printf("%-10s %-20s %-20s %-20s %-15s\n", "Port Name", "WWPN", "WWNN", "Location Code", "Available Ports")
		fmt.Println("-------------------------------------------------------------------------")
		
		for _, port := range fcPorts {
			fmt.Printf("%-10s %-20s %-20s %-20s %-15s\n",
				port.PortName,
				port.WWPN,
				port.WWNN,
				port.LocationCode,
				port.AvailablePorts,
			)
		}
		
		fmt.Println("=========================================================================")
		
		// Display additional details
		fmt.Println("\n📋 Port Details:")
		for i, port := range fcPorts {
			fmt.Printf("\n[%d] Port: %s\n", i+1, port.PortName)
			fmt.Printf("    WWPN:           %s\n", port.WWPN)
			fmt.Printf("    WWNN:           %s\n", port.WWNN)
			fmt.Printf("    Location Code:  %s\n", port.LocationCode)
			fmt.Printf("    Available Ports: %s\n", port.AvailablePorts)
			fmt.Printf("    Total Ports:    %s\n", port.TotalPorts)
			fmt.Printf("    Device ID:      %s\n", port.UniqueDeviceID)
		}
	}

	fmt.Println("\n🎉 Physical FC Port retrieval complete!")
}

// Made with Bob
