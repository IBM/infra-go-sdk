package main

import (
	"log"
	"os"

	hmc "github.com/sudeeshjohn/PowerHMC" // Replace with your actual hmc package import path
)

func main() {
	// Define command-line flags
	hmcIP := "192.0.2.3"
	username := "REDACTED_HMC_USER<=="
	password := "REDACTED_HMC_PASS<=="
	verbose := true
	viosUUID := "0625F241-08C9-461D-9FA6-B46620D6FDB1"
	lparUUID := "6E20E53A-28F8-4D04-92D2-B32236C4B37A"
	volumeName := "hdisk1"

	// Initialize HmcRestClient (assuming NewHmcRestClient is in hmc package and sets up insecure TLS)
	restClient := hmc.NewHmcRestClient(hmcIP)

	// Perform login
	if verbose {
		log.Printf("Attempting to log on to HMC at %s with username %s", hmcIP, username)
	}
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}

	// Remove the volume-LPAR mapping
	_,_,err := restClient.RemoveVolumeLPARMapping(viosUUID, lparUUID, volumeName, verbose)
	if err != nil {
		log.Fatalf("Failed to remove volume-LPAR mapping: %v", err)
	}

	log.Printf("Successfully removed mapping for volume %s on LPAR %s from VIOS %s", volumeName, lparUUID, viosUUID)

	os.Exit(0)
}
