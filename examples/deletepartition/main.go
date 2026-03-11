package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	hmc "github.com/sudeeshjohn/PowerHMC"
)

func main() {
	// --- Configuration ---
	hmcIP      := "192.0.2.1"
	username   := "REDACTED_HMC_USER<=="
	password   := "REDACTED_HMC_PASS<=="
	sysName    := "LTC13U29-Ranier" // Managed System Name
	lparName   := "test-test"       // LPAR Name to delete
	verbose    := true

	// 1. Initialize Client
	restClient := hmc.NewHmcRestClient(hmcIP)

	// 2. Logon
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff()

	// 3. Find Managed System UUID
	fmt.Printf("Step 1: Locating System [%s]...\n", sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(sysName, verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("System %s not found: %v", sysName, err)
	}

	// 4. Find LPAR UUID and Current State
	fmt.Printf("Step 2: Locating Partition [%s] on System [%s]...\n", lparName, sysName)
	// Using your updated function name
	lpars, err := restClient.GetLogicalPartitionsQuickAll(sysUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to fetch partitions: %v", err)
	}

	var targetLparUUID string
	var currentState string

	for _, l := range lpars {
		if l.PartitionName == lparName {
			targetLparUUID = l.UUID
			currentState = strings.ToLower(l.PartitionState) 
			break
		}
	}

	if targetLparUUID == "" {
		log.Fatalf("Error: Partition %s not found on system %s", lparName, sysName)
	}

	// 5. Shutdown if not in "not activated" state
	// Based on your JSON, valid running state is "running"
	if currentState != "not activated" {
		fmt.Printf("Step 3: Partition is currently '%s'. Shutting down...\n", currentState)
		
		// shutdownOption: "Immediate", restart: false
		_, err := restClient.PowerOffPartition(sysUUID, targetLparUUID, "Immediate", false, verbose)
		if err != nil {
			log.Fatalf("Power off failed: %v", err)
		}
		fmt.Println("Shutdown job finished successfully.")
		
		// Small buffer to allow HMC state synchronization
		time.Sleep(5 * time.Second)
	} else {
		fmt.Println("Step 3: Partition is already 'not activated'. Skipping shutdown.")
	}

	// 6. Delete the Partition
	fmt.Printf("Step 4: Deleting Partition [%s] (UUID: %s)...\n", lparName, targetLparUUID)
	err = restClient.DeleteLogicalPartition(targetLparUUID, verbose)
	if err != nil {
		log.Fatalf("Delete failed: %v", err)
	}

	fmt.Printf("\nSUCCESS: Managed System '%s' -> Partition '%s' has been deleted.\n", sysName, lparName)
}