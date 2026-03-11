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
	verbose := true
	partUUID := "67C1301A-87D2-4ECC-A8D2-23094162BC5A"

	//systemUUID := "49672f05-253d-30bc-ae09-ecd76cb410ce"

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
	err := restClient.DeleteLogicalPartition(partUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to retrieve partition properties: %v", err)
	}
	fmt.Printf("Partition ID: %s is deleted\n", partUUID)

}
