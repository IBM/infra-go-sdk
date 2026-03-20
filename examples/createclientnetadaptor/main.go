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
	sysUUID := "321a1ec0-49a9-3ba0-ba52-bceebaf1c607"            // LTC09U31-ZZ
	lparUUID := "48FFBFB4-2DB8-448E-A33C-2C5A86D9CE17"           // Go_LPAR_01
	vswitchUUID := "ab6312d3-c010-3dab-aba1-66ba447b8cc6"        // VNET0
	vlanID := 1

	// 3. Create the Adapter
	fmt.Printf("🚀 Adding VLAN %d (Switch: VNET0) to LPAR %s...\n", vlanID, lparUUID)
	adapterUUID, err := client.CreateClientNetworkAdapter(sysUUID, lparUUID, vswitchUUID, vlanID, true)
	if err != nil {
		log.Fatalf("❌ Failed to add adapter: %v", err)
	}

	fmt.Printf("\n✨ Successfully provisioned Virtual Ethernet Adapter!\nNew Adapter UUID: %s\n", adapterUUID)
}
