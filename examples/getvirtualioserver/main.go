package main

import (
	"encoding/json"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your package path if necessary
)

func main() {
	// --- Configuration ---
	hmcIP        := "192.0.2.1"
	username     := "REDACTED_HMC_USER<=="
	password     := "REDACTED_HMC_PASS<=="
	targetSystem := "LTC09U31-ZZ" // The name of the managed system 
	verbose      := false

	// 1. Initialize and Login
	restClient := hmc.NewHmcRestClient(hmcIP)
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff()

	fmt.Println("Successfully authenticated with HMC.")

	// 2. Fetch the Quick list to resolve the Managed System UUID
	fmt.Printf("Resolving UUID for Managed System '%s'...\n", targetSystem)
	systems, err := restClient.GetManagedSystemQuickAll(verbose)
	if err != nil {
		log.Fatalf("Error retrieving systems: %v", err)
	}

	var systemUUID string
	for _, s := range systems {
		if s.SystemName == targetSystem {
			systemUUID = s.UUID
			break
		}
	}

	if systemUUID == "" {
		log.Fatalf("Target system '%s' not found in the managed systems list.", targetSystem)
	}
	fmt.Printf("Found Managed System UUID: %s\n", systemUUID)

	// 3. Get the list of VIOSs using your existing Quick method to grab a valid VIOS UUID
	fmt.Println("Fetching Quick VIOS list to find a target VIOS UUID...")
	quickViosList, err := restClient.GetVirtualIOServersQuick(systemUUID, verbose)
	if err != nil {
		log.Fatalf("Error fetching VIOS list: %v", err)
	}

	if len(quickViosList) == 0 {
		fmt.Println("No Virtual I/O Servers found on this managed system.")
		return
	}

	// For this test, we will just pick the very first VIOS in the list
	targetViosUUID := quickViosList[0].UUID
	fmt.Printf("Selected Target VIOS UUID: %s\n", targetViosUUID)

	// 4. Fetch the Comprehensive Details for this SINGLE VIOS
	fmt.Println("\nFetching detailed information for the specific VIOS via REST API...")
	viosDetails, err := restClient.GetVirtualIOServer(targetViosUUID, verbose)
	if err != nil {
		log.Fatalf("Error fetching specific VIOS details: %v", err)
	}

	// 5. Pretty-print the results to the console
	fmt.Printf("\n======================================================\n")
	fmt.Printf(" DETAILED VIOS INFO: %s (Partition ID: %s)\n", viosDetails.PartitionName, viosDetails.PartitionID)
	fmt.Printf("======================================================\n")

	output, err := json.MarshalIndent(viosDetails, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal VIOS data to JSON: %v", err)
	}

	fmt.Println(string(output))
			fmt.Println("  Fibre Channel Ports (WWPNs):")
		for _, fcPort := range viosDetails.Storage.FibreChannelPorts {
			// Printing Port Name alongside WWPN so you know which port the WWPN belongs to
			fmt.Printf("  - Port: %-5s | WWPN: %s\n", fcPort.PortName, fcPort.WWPN)
		}
}
