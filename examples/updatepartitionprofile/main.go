package main

import (
	"context"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	hmcIP := "192.0.2.4"
	username := "REDACTED_HMC_USER<=="
	password := "REDACTED_HMC_PASS<=="
	verbose := false
	partitionUUID := "6C7FFA07-5A4A-4545-97FF-37732EE54523" // Replace with actual partition UUID
	profileName := "default_profile"
	//profileUUID := "6b2fc41c-be7b-30c5-87e7-e39d9eb96c3e"    // Replace with actual profile UUID

	  // Sample updated profile XML (only the profile body)
    updatedProfile := `
<LogicalPartitionProfile xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" schemaVersion="V1_4_0">
    <PartitionName>LPAR1</PartitionName>
    <DesiredProcessors>2</DesiredProcessors>
    <DesiredMemory>4194304</DesiredMemory>
</LogicalPartitionProfile>
`

    // Initialize HmcRestClient
	restClient := hmc.NewHmcRestClient(hmcIP)

	// Logon
	if verbose {
		log.Printf("Attempting to log on to HMC at %s with username %s", hmcIP, username)
	}
	if err := restClient.Login(context.Background(), username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer func() {
		if err := restClient.Logoff(context.Background()); err != nil {
			log.Printf("Logoff failed: %v", err)
		} else if verbose {
			log.Println("Logged off successfully")
		}
	}()

    err := restClient.UpdateLogicalPartitionProfile(partitionUUID, profileName, updatedProfile, verbose)
    if err != nil {
        log.Fatalf("Failed to update logical partition profile: %v", err)
    }

    fmt.Println("Profile updated successfully")
}