package main

import (
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	hmcIP := "192.0.2.3"
	username := "REDACTED_HMC_USER<=="
	password := "REDACTED_HMC_PASS<=="
	verbose := true
	partUUID := "6E20E53A-28F8-4D04-92D2-B32236C4B37A"
	profileUUID := "64942733-5f93-3569-9d33-428d0e4e4270"

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
	partition, err := restClient.PowerOnPartition(partUUID, profileUUID, "normal", "", "", verbose)
	if err != nil {
		log.Fatalf("Failed to retrieve partition properties: %v", err)
	}
	fmt.Printf("Partition ID: %s; Partitoin Name: %s\n", partition.GetPath(), partition.GetPath())

}
