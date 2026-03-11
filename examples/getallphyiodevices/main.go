package main

import (
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/PowerHMC"
)

func main() {
	hmcIP := "192.0.2.4"
	username := "REDACTED_HMC_USER<=="
	password := "REDACTED_HMC_PASS<=="
	verbose := false
	systemUUID := "49672f05-253d-30bc-ae09-ecd76cb410ce"

	// Initialize HmcRestClient
	restClient := hmc.NewHmcRestClient(hmcIP)

	// Logon
	if verbose {
		log.Printf("Attempting to log on to HMC at %s with username %s", hmcIP, username)
	}
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer func() {
		if err := restClient.Logoff(); err != nil {
			log.Printf("Logoff failed: %v", err)
		} else if verbose {
			log.Println("Logged off successfully")
		}
	}()

	// Retrieve managed system info (IO adapters)
	adapters, err := restClient.GetManagedSystemInfo(systemUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to retrieve managed system info: %v", err)
	}

	// Parse the adapters
	adapterInfo := hmc.ParseIOAdapters(adapters)

	// Print the parsed info
	fmt.Printf("Found %d IO adapters:\n", len(adapterInfo))
	for i, info := range adapterInfo {
		fmt.Printf("Adapter %d:\n", i+1)
		fmt.Printf("  Description: %s\n", info.Description)
		fmt.Printf("  LogicalPartitionAssignmentCapable: %t\n", info.LogicalPartitionAssignmentCapable)
		fmt.Printf("  DeviceName: %s\n", info.DeviceName)
	}
}