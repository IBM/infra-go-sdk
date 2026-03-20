package main

import (
	"fmt"
	"log"
	"strings"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	client := hmc.NewHmcRestClient("192.0.2.1")
	client.Login("REDACTED_HMC_USER<==", "REDACTED_HMC_PASS<==", false)
	defer client.Logoff()

	lparUUID := "48FFBFB4-2DB8-448E-A33C-2C5A86D9CE17"

	fmt.Println("Fetching Client Network Adapters...")
	adapters, err := client.GetClientNetworkAdapters(lparUUID, true)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	for i, a := range adapters {
		fmt.Printf("\n--- Adapter %d ---\n", i+1)
		fmt.Printf("UUID:           %s\n", a.UUID)
		fmt.Printf("MAC Address:    %s\n", a.MACAddress)
		fmt.Printf("Port VLAN ID:   %s\n", a.PortVLANID)
		fmt.Printf("Virtual Switch: %s (ID: %s)\n", a.VirtualSwitchName, a.VirtualSwitchID)
		fmt.Printf("Virtual Slot:   %s\n", a.VirtualSlotNumber)
		fmt.Printf("Location Code:  %s\n", a.LocationCode)
		
		fmt.Println("Attached Networks:")
		for _, net := range a.VirtualNetworkURIs {
			fmt.Printf("  - %s\n", net)
		}
		fmt.Println(strings.Repeat("-", 40))
	}
}
