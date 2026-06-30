package main

import (
	"flag"
	"context"
	"encoding/json"
	"fmt"
	"log"

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

	targetSystem := "" // The name of the managed system
	// 1. Initialize and Login
	restClient := exutil.NewClient(hmcIPVal, *debug, *debugFull)
	if err := restClient.Login(context.Background(), usernameVal, passwordVal); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	fmt.Println("Successfully authenticated with HMC.")

	// 2. Fetch the Quick list to resolve the Managed System UUID
	fmt.Printf("Resolving UUID for Managed System '%s'...\n", targetSystem)
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
	fmt.Printf("Found Managed System UUID: %s\n", systemUUID)

	// 3. Get the list of VIOSs using your existing Quick method to grab a valid VIOS UUID
	fmt.Println("Fetching Quick VIOS list to find a target VIOS UUID...")
	quickViosList, err := restClient.GetVirtualIOServersQuick(context.Background(), systemUUID)
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
	viosDetails, err := restClient.GetVirtualIOServer(context.Background(), targetViosUUID)
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
