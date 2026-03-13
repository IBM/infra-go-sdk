package main

import (
	"encoding/json"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/PowerHMC" // Adjust to your package path if necessary
)

func main() {
	// --- Configuration ---
	hmcIP        := "192.0.2.1"
	username     := "REDACTED_HMC_USER<=="
	password     := "REDACTED_HMC_PASS<=="
	targetSystem := "LTC09U31-ZZ" // The name of the managed system we want to query
	verbose      := false

	// 1. Initialize and Login
	restClient := hmc.NewHmcRestClient(hmcIP)
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff()

	fmt.Println("Successfully authenticated with HMC.")

	// 2. Fetch the Quick list to resolve the UUID for targetSystem
	fmt.Println("Fetching Managed Systems inventory to resolve UUID...")
	systems, err := restClient.GetManagedSystemsQuick(verbose)
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

	fmt.Printf("Found Managed System '%s' with UUID: %s\n", targetSystem, systemUUID)

	// 3. Fetch the Comprehensive VIOS Details using the dynamically resolved UUID
	fmt.Println("Fetching Virtual I/O Servers...")
	viosList, err := restClient.GetVirtualIOServers(systemUUID, verbose)
	if err != nil {
		log.Fatalf("Error fetching VIOS details: %v", err)
	}

	if len(viosList) == 0 {
		fmt.Println("No Virtual I/O Servers found on this managed system.")
		return
	}

	fmt.Printf("Successfully retrieved %d Virtual I/O Server(s).\n", len(viosList))

	// 4. Print the details separately for each VIOS
	for i, vios := range viosList {
		fmt.Printf("\n======================================================\n")
		fmt.Printf(" VIOS %d: %s (Partition ID: %s)\n", i+1, vios.PartitionName, vios.PartitionID)
		fmt.Printf("======================================================\n")

		// Marshal just this specific VIOS struct into formatted JSON
		output, err := json.MarshalIndent(vios, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal VIOS data to JSON for %s: %v\n", vios.PartitionName, err)
			continue
		}

		fmt.Println(string(output))
	}
	// 4. Print ONLY the Fibre Channel Ports and WWPN for each VIOS
	for i, vios := range viosList {
		fmt.Printf("\n======================================================\n")
		fmt.Printf(" VIOS %d: %s (Partition ID: %s)\n", i+1, vios.PartitionName, vios.PartitionID)
		fmt.Printf("======================================================\n")

		// Check if there are any FC ports available
		if len(vios.Storage.FibreChannelPorts) == 0 {
			fmt.Println("  No Fibre Channel Ports found on this VIOS.")
			continue
		}

		fmt.Println("  Fibre Channel Ports (WWPNs):")
		for _, fcPort := range vios.Storage.FibreChannelPorts {
			// Printing Port Name alongside WWPN so you know which port the WWPN belongs to
			fmt.Printf("  - Port: %-5s | WWPN: %s\n", fcPort.PortName, fcPort.WWPN)
		}
	}
}