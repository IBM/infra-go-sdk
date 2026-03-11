package main

import (
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/PowerHMC"
)

func main() {
	hmcIP := "192.0.2.3"
	username := "REDACTED_HMC_USER<=="
	password := "REDACTED_HMC_PASS<=="
	verbose := false
	partUUID := "0DE0C178-F78D-4965-8C7C-B25E37DD44D1"

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

	// Retrieve partition properties
	partition, err := restClient.GetLogicalPartitionQuick(partUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to retrieve partition properties: %v", err)
	}
	fmt.Printf("Partition ID: %s; Partitoin Name: %s\n", partition.UUID, partition.PartitionName)

}
