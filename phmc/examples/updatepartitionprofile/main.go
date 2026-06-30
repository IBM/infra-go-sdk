package main

import (
	"flag"
	"context"
	"fmt"
	"log"

	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

func main() {
	hmcIP    := flag.String("hmc-ip",    "", "HMC IP address")
	username := flag.String("hmc-user",  "", "HMC username")
	password := flag.String("hmc-pass",  "", "HMC password")
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()

	hmcIPVal    := *hmcIP
	usernameVal := *username
	passwordVal := *password

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
	restClient := exutil.NewClient(hmcIPVal, *debug, *debugFull)

	// Logon
	if verbose {
		log.Printf("Attempting to log on to HMC at %s with username %s", hmcIP, username)
	}
	if err := restClient.Login(context.Background(), usernameVal, passwordVal); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer func() {
		if err := restClient.Logoff(context.Background()); err != nil {
			log.Printf("Logoff failed: %v", err)
		} else if verbose {
			log.Println("Logged off successfully")
		}
	}()

    err := restClient.UpdateLogicalPartitionProfile(partitionUUID, profileName, updatedProfile)
    if err != nil {
        log.Fatalf("Failed to update logical partition profile: %v", err)
    }

    fmt.Println("Profile updated successfully")
}
