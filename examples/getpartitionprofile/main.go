package main

import (
	"fmt"
	"log"

	"github.com/beevik/etree"
	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	hmcIP := "192.0.2.4"
	username := "REDACTED_HMC_USER<=="
	password := "REDACTED_HMC_PASS<=="
	verbose := false
	partitionUUID := "6C7FFA07-5A4A-4545-97FF-37732EE54523" // Replace with actual partition UUID

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

	// Retrieve partition profiles
	profiles, err := restClient.GetLogicalPartitionProfiles(partitionUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to retrieve partition profiles: %v", err)
	}

	doc := etree.NewDocument()
	doc.SetRoot(profiles[0].Copy()) // Assuming at least one profile; adjust as needed

	xmlStr, err := doc.WriteToString()
	if err != nil {
		fmt.Printf("Error serializing XML: %v\n", err)
		return
	}

	fmt.Println(xmlStr)
}