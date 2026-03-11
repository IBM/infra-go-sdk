package main

import (
	"log"

	hmc "github.com/sudeeshjohn/PowerHMC"
)

func main() {
	hmcIP := "192.0.2.3"
	username := "REDACTED_HMC_USER<=="
	password := "REDACTED_HMC_PASS<=="
	verbose := true
	viosUUID := "0625F241-08C9-461D-9FA6-B46620D6FDB1"

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

	// Get free physical volumes for the VIOS
	pvList, err := restClient.GetFreePhyVolume(viosUUID, verbose)
	if err != nil {
		// Log the error and assume no volumes are available
		if verbose {
			log.Printf("Error getting free physical volumes for VIOS %s: %v", pvList, err)
		}
		pvList = []hmc.PhysicalVolume{} // Treat as no volumes found
	}

}
