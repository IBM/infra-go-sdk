package main

import (
	"context"
	"fmt"
	"log"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc"
)

func main() {
	client := hmc.NewRestClient("192.0.2.1")
	if err := client.Login(context.Background(), "REDACTED_HMC_USER<==", "REDACTED_HMC_PASS<==", true); err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	defer client.Logoff(context.Background())

	lparUUID := "48FFBFB4-2DB8-448E-A33C-2C5A86D9CE17"
	viosID := 1      // Target VIOS ID
	viosSlot := 49    // An AVAILABLE slot number on that VIOS

	fmt.Printf("🚀 Provisioning vSCSI Client Adapter mapped to VIOS %d, Slot %d...\n", viosID, viosSlot)
	adapterUUID, err := client.CreateVirtualSCSIClientAdapter(lparUUID, viosID, viosSlot, true)
	if err != nil {
		log.Fatalf("❌ Failed: %v", err)
	}

	fmt.Printf("\n✨ Success! Adapter UUID: %s\n", adapterUUID)
}