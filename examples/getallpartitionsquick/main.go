package main

import (
	"fmt"
	"log"
	"strings"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	// --- Configuration ---
	hmcIP      := "192.0.2.1"
	username   := "REDACTED_HMC_USER<=="
	password   := "REDACTED_HMC_PASS<=="
	//targetSystem := "LTC09U31-ZZ" // The name of the managed system
	targetSystem := "LTC09U31-ZZ" // The name of the managed system
	verbose    := false 

	// 1. Initialize Client
	restClient := hmc.NewHmcRestClient(hmcIP)

	// 2. Logon
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff()

	// 3. Find Managed System UUID by Name
	fmt.Printf("Searching for Managed System: %s...\n", targetSystem)
	sysUUID, _, err := restClient.GetManagedSystemByName(targetSystem, verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("Could not find system %s: %v", targetSystem, err)
	}
	fmt.Printf("Found System UUID: %s\n\n", sysUUID)

	// 4. Get all Logical Partitions using the Quick/All endpoint
	// This uses the optimized JSON streaming you just added to logicalpartitions.go
	partitions, err := restClient.GetLogicalPartitionsQuickAll(sysUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to fetch partitions: %v", err)
	}

	// 5. Display the list in a clean table
	fmt.Printf("Partitions on %s:\n", targetSystem)
	fmt.Printf("%-3s | %-4s | %-20s | %-15s | %-12s\n", "No", "ID", "Name", "State", "OS Type")
	fmt.Println(strings.Repeat("-", 65))

	for i, p := range partitions {
		fmt.Printf("%-3d | %-4d | %-20s | %-15s | %-12s | %-15s\n", 
			i+1, 
			p.PartitionID, 
			p.PartitionName, 
			p.PartitionState, 
			p.OperatingSystemType,
			p.UUID,
		)
	}
}
