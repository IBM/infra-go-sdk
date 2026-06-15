package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc" // Adjust to your package path if necessary
)

func main() {
	// --- Configuration ---
	hmcIP        := ""
	username     := ""
	password     := ""
	targetSystem := "" // The name of the managed system
	verbose      := false

	// 1. Initialize and Login
	restClient := hmc.NewRestClient(hmcIP)
	if err := restClient.Login(context.Background(), username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	fmt.Println("Successfully authenticated with HMC.")

	// 2. Fetch the Quick list to resolve the Managed System UUID
	fmt.Printf("Resolving UUID for Managed System '%s'...\n", targetSystem)
	systems, err := restClient.GetManagedSystemQuickAll(context.Background(), verbose)
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
	quickViosList, err := restClient.GetVirtualIOServersQuick(context.Background(), systemUUID, verbose)
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
	viosDetails, err := restClient.GetVirtualIOServer(context.Background(), targetViosUUID, verbose)
	if err != nil {
		log.Fatalf("Error fetching specific VIOS details: %v", err)
	}

	// 5. Pretty-print the results to the console
	fmt.Printf("\n======================================================\n")
	fmt.Printf(" DETAILED VIOS INFO: %s (Partition ID: %d)\n", viosDetails.PartitionName, viosDetails.PartitionID)
	fmt.Printf("======================================================\n")

	output, err := json.MarshalIndent(viosDetails, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal VIOS data to JSON: %v", err)
	}

	fmt.Println(string(output))
	
	// 6. Display Fibre Channel Ports (WWPNs) if available
	fmt.Println("\n  Fibre Channel Ports (WWPNs):")
	fcPortFound := false
	for _, profileSlot := range viosDetails.PartitionIOConfiguration.ProfileIOSlots {
		fcAdapter := profileSlot.AssociatedIOSlot.RelatedIOAdapter.PhysicalFibreChannelAdapter
		if len(fcAdapter.PhysicalFibreChannelPorts) > 0 {
			for _, fcPort := range fcAdapter.PhysicalFibreChannelPorts {
				// Printing Port Name alongside WWPN so you know which port the WWPN belongs to
				fmt.Printf("    - Port: %-5s | WWPN: %s\n", fcPort.PortName, fcPort.WWPN)
				fcPortFound = true
			}
		}
	}
	if !fcPortFound {
		fmt.Println("    No Fibre Channel ports found")
	}
}
