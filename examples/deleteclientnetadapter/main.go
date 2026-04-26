package main

import (
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// 1. Connection Details
	client := hmc.NewHmcRestClient("192.0.2.1")
	if err := client.Login("REDACTED_HMC_USER<==", "REDACTED_HMC_PASS<==", true); err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	defer client.Logoff()

	// 2. Resource UUIDs
	lparUUID := "48FFBFB4-2DB8-448E-A33C-2C5A86D9CE17"           // Go_LPAR_01
	adapterUUID := "fba639de-886a-3696-91e1-4feb8caeaf79"        // Replace with your actual Adapter UUID

	// 3. Delete the Adapter
	fmt.Printf("🗑️ Deleting Virtual Ethernet Adapter %s...\n", adapterUUID)
	err := client.DeleteClientNetworkAdapter(lparUUID, adapterUUID, true)
	if err != nil {
		log.Fatalf("❌ Delete failed: %v", err)
	}

	fmt.Println("✨ Successfully deleted the adapter!")
}
