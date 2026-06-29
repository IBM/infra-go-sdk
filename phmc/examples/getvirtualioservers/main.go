package main

import (
	"flag"
	"context"
	"encoding/json"
	"fmt"
	"log"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your package path if necessary
	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

func main() {
	// --- Configuration ---
	hmcIP    := flag.String("hmc-ip",    "", "HMC IP address")
	username := flag.String("hmc-user",  "", "HMC username")
	password := flag.String("hmc-pass",  "", "HMC password")
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()

	hmcIPVal    := *hmcIP
	usernameVal := *username
	passwordVal := *password

	targetSystem := "" // The name of the managed system we want to query
	// 1. Initialize and Login
	restClient := exutil.NewClient(hmcIPVal, *debug, *debugFull)
	if err := restClient.Login(context.Background(), usernameVal, passwordVal); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	fmt.Println("Successfully authenticated with HMC.")

	// 2. Fetch the Quick list to resolve the UUID for targetSystem
	fmt.Println("Fetching Managed Systems inventory to resolve UUID...")
	systems, err := restClient.GetManagedSystemQuickAll(context.Background())
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
	viosList, err := restClient.GetVirtualIOServers(systemUUID)
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
		fmt.Printf(" VIOS %d: %s (Partition ID: %d)\n", i+1, vios.PartitionName, vios.PartitionID)
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
		fmt.Printf(" VIOS %d: %s (Partition ID: %d)\n", i+1, vios.PartitionName, vios.PartitionID)
		fmt.Printf("======================================================\n")
	
		// Collect all FC ports from the nested structure
		var fcPorts []hmc.PhysicalFibreChannelPort
		for _, profileSlot := range vios.PartitionIOConfiguration.ProfileIOSlots {
			adapter := profileSlot.AssociatedIOSlot.RelatedIOAdapter.PhysicalFibreChannelAdapter
			if len(adapter.PhysicalFibreChannelPorts) > 0 {
				fcPorts = append(fcPorts, adapter.PhysicalFibreChannelPorts...)
			}
		}
		
		// Check if there are any FC ports available
		if len(fcPorts) == 0 {
			fmt.Println("  No Fibre Channel Ports found on this VIOS.")
			continue
		}
	
		fmt.Println("  Fibre Channel Ports (WWPNs):")
		for _, fcPort := range fcPorts {
			// Printing Port Name alongside WWPN so you know which port the WWPN belongs to
			fmt.Printf("  - Port: %-5s | WWPN: %s\n", fcPort.PortName, fcPort.WWPN)
		}
	}
}
