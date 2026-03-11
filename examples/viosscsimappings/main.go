package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	// Assuming this is used for XML parsing; adjust if needed

	hmc "github.com/sudeeshjohn/PowerHMC"
)

func main() {
	// Define command-line flags
	hmcIP := "192.0.2.3"
	username := "REDACTED_HMC_USER<=="
	password := "REDACTED_HMC_PASS<=="
	verbose := true
	viosUUID := "0625F241-08C9-461D-9FA6-B46620D6FDB1"

	// Initialize HmcRestClient
	restClient := hmc.NewHmcRestClient(hmcIP)

	// Perform login
	if verbose {
		log.Printf("Attempting to log on to HMC at %s with username %s", hmcIP, username)
	}
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}

	// Fetch VSCSI mappings
	mappings, err := restClient.GetViosSCSIMappings(viosUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to get VSCSI mappings: %v", err)
	}
	// Output the mappings (e.g., print details)
	fmt.Printf("Found %d VSCSI mappings:\n", len(mappings))

	for i, mapping := range mappings {
		// Extract and print relevant details; adjust paths based on your XML structure
		clientSlot := mapping.FindElement("ClientAdapter/VirtualSlotNumber")
		serverSlot := mapping.FindElement("ServerAdapter/VirtualSlotNumber")
		targetName := mapping.FindElement("TargetDevice/PhysicalVolumeVirtualTargetDevice/TargetName")
		backingVolumeName := mapping.FindElement("Storage/PhysicalVolume/VolumeName")
		associatedLogicalPartition := mapping.FindElement("//AssociatedLogicalPartition")

		clientSlotText := ""
		if clientSlot != nil {
			clientSlotText = clientSlot.Text()
		}
		serverSlotText := ""
		if serverSlot != nil {
			serverSlotText = serverSlot.Text()
		}
		targetNameText := ""
		if targetName != nil {
			targetNameText = targetName.Text()
		}
		backingVolumeText := ""
		if backingVolumeName != nil {
			backingVolumeText = backingVolumeName.Text()
		}
		associatedPartitionUUID := ""
		if associatedLogicalPartition != nil {
			// get the href attribute

			assopartitionText := associatedLogicalPartition.SelectAttrValue("href", "")
			parts := strings.Split(assopartitionText, "/")
			associatedPartitionUUID = parts[len(parts)-1]

		}

		fmt.Printf("Mapping %d:\n", i+1)
		fmt.Printf("  Client Slot: %s\n", clientSlotText)
		fmt.Printf("  Server Slot: %s\n", serverSlotText)
		fmt.Printf("  Target Device Name: %s\n", targetNameText)
		fmt.Printf("  Backing Volume Name: %s\n", backingVolumeText)
		fmt.Printf("  Assiciated Partition UUID: %s\n", associatedPartitionUUID)

	}
	/* for i, m := range mappings {
		doc := etree.NewDocument()
		doc.SetRoot(m) // attach the element as the root
		s, _ := doc.WriteToString()
		fmt.Printf("Mapping %d:\n%s\n", i, s)
	} */
	os.Exit(0)
}
