package main

import (
	"flag"
	"context"
	"fmt"
	"log"
	"strings"

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
	// 1. Initialize Client
	restClient := exutil.NewClient(hmcIPVal, *debug, *debugFull)

	// 2. Logon
	if err := restClient.Login(context.Background(), usernameVal, passwordVal); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// 3. Find Managed System UUID by Name
	fmt.Printf("Searching for Managed System: %s...\n", targetSystem)
	sysUUID, _, err := restClient.GetManagedSystemByName(context.Background(), targetSystem)
	if err != nil || sysUUID == "" {
		log.Fatalf("Could not find system %s: %v", targetSystem, err)
	}
	fmt.Printf("Found System UUID: %s\n\n", sysUUID)

	// 4. Get all Logical Partitions using the Quick/All endpoint
	// This uses the optimized JSON streaming you just added to logicalpartitions.go
	partitions, err := restClient.GetLogicalPartitionsQuickAll(context.Background(), sysUUID)
	if err != nil {
		log.Fatalf("Failed to fetch partitions: %v", err)
	}

	// 5. Display the list in a clean table
	fmt.Printf("Partitions on %s:\n", targetSystem)
	fmt.Printf("%-3s | %-4s | %-20s | %-15s | %-12s\n", "No", "ID", "Name", "State", "OS Type")
	fmt.Println(strings.Repeat("-", 65))

	for i, p := range partitions {
		fmt.Printf("%-3d | %-4d | %-20s | %-15s | %-12s\n", 
			i+1, 
			p.PartitionID, 
			p.PartitionName, 
			p.PartitionState, 
			p.OperatingSystemType,
		)
	}
}
