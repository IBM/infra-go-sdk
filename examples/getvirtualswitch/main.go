package main

import (
	"fmt"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	client := hmc.NewHmcRestClient("192.0.2.1")
	client.Login("REDACTED_HMC_USER<==", "REDACTED_HMC_PASS<==", false)
	defer client.Logoff()

	sysUUID := "321a1ec0-49a9-3ba0-ba52-bceebaf1c607"

	// 1. Test Quick All
	fmt.Println("--- Quick All ---")
	quickAll, _ := client.GetVirtualSwitchQuickAll(sysUUID, false)
	for _, s := range quickAll {
		fmt.Printf("Name: %s | UUID: %s\n", s.SwitchName, s.UUID)
	}

	// 2. Test Quick Singular
	fmt.Println("\n--- Quick Singular ---")
	if len(quickAll) > 0 {
		quickOne, _ := client.GetVirtualSwitchQuick(sysUUID, quickAll[0].UUID, false)
		fmt.Printf("Fetched Singular: %s (Mode: %s)\n", quickOne.SwitchName, quickOne.SwitchMode)
	}

	// 3. Test XML Comprehensive
	fmt.Println("\n--- Comprehensive XML Feed ---")
	xmlSwitches, _ := client.GetVirtualSwitches(sysUUID, false)
	for _, s := range xmlSwitches {
		fmt.Printf("Switch: %s (ID: %s)\n", s.SwitchName, s.SwitchID)
		for i, net := range s.VirtualNetworks {
			fmt.Printf("  └─ Attached Network %d URI: %s\n", i+1, net)
		}
	}
}
