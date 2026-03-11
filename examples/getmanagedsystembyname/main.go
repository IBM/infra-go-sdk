package main

import (
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/PowerHMC"
)

func main() {
	hmcIP      := "192.0.2.1"
	username   := "REDACTED_HMC_USER<=="
	password   := "REDACTED_HMC_PASS<=="
	targetName := "LTC13U05"
	verbose    := false

	restClient := hmc.NewHmcRestClient(hmcIP)
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff()

	fmt.Printf("Searching for Managed System: %s...\n", targetName)
	
	uuid, msElem, err := restClient.GetManagedSystemByName(targetName, verbose)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	if msElem == nil {
		fmt.Println("System not found.")
		return
	}

	// SUCCESSFUL EXTRACTION
	fmt.Println("\n--- System Details ---")
	fmt.Printf("UUID:  %s\n", uuid)
	
	// Use the element we already have to avoid the extra API call that caused the 500
	maxPartitions := msElem.FindElement("MaximumPartitions")
	state         := msElem.FindElement("State")
	model  := msElem.FindElement("MachineTypeModelAndSerialNumber/MachineType")
	serial := msElem.FindElement("MachineTypeModelAndSerialNumber/SerialNumber")
	fw     := msElem.FindElement("ActivatedFirmwareLevel")

	if state != nil {
		fmt.Printf("State: %s\n", state.Text())
	}
	if model != nil && serial != nil {
		fmt.Printf("MTMS:   %s*%s\n", model.Text(), serial.Text())
	}
	if fw != nil {
		fmt.Printf("FW:     %s\n", fw.Text())
	}

	if maxPartitions != nil {
		fmt.Printf("Max Partitions: %s\n", maxPartitions.Text())
	} else {
		// Sometimes it's nested under 'AssociatedSystemProperties'
		altPath := msElem.FindElement(".//MaximumPartitions")
		if altPath != nil {
			fmt.Printf("Max Partitions (found via deep search): %s\n", altPath.Text())
		} else {
			fmt.Println("Max Partitions: Not found in XML")
		}
	}
}

