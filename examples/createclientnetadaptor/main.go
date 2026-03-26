package main

import (
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// Command-line flags for dynamic configuration
	hmcIP := flag.String("hmc", "192.0.2.1", "HMC IP address")
	username := flag.String("user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("pass", "REDACTED_HMC_PASS<==", "HMC password")
	systemName := flag.String("system", "LTC09U31-ZZ", "Managed System name (required)")
	lparName := flag.String("lpar", "Go_LPAR_100", "LPAR name (required)")
	vswitchName := flag.String("vswitch", "ETHERNET0(Default)", "Virtual Switch name (default: VNET0)")
	vlanID := flag.Int("vlan", 1337, "VLAN ID (default: 1)")
	verbose := flag.Bool("verbose", true, "Enable verbose logging")
	
	flag.Parse()

	// Validate required parameters
	if *systemName == "" {
		log.Fatal("❌ Error: --system parameter is required")
	}
	if *lparName == "" {
		log.Fatal("❌ Error: --lpar parameter is required")
	}

	// 1. Connect to HMC
	fmt.Printf("🔌 Connecting to HMC at %s...\n", *hmcIP)
	client := hmc.NewHmcRestClient(*hmcIP)
	if err := client.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("❌ Login failed: %v", err)
	}
	defer client.Logoff()
	fmt.Println("✅ Successfully connected to HMC")

	// 2. Get Managed System UUID by name
	fmt.Printf("\n🔍 Looking up Managed System: %s...\n", *systemName)
	sysUUID, _, err := client.GetManagedSystemByName(*systemName, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get system UUID: %v", err)
	}
	if sysUUID == "" {
		log.Fatalf("❌ Managed System '%s' not found", *systemName)
	}
	fmt.Printf("✅ Found System UUID: %s\n", sysUUID)

	// 3. Get all LPARs in the system and find the target LPAR
	fmt.Printf("\n🔍 Looking up LPAR: %s...\n", *lparName)
	lpars, err := client.GetLogicalPartitionsQuickAll(sysUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get LPARs: %v", err)
	}

	var lparUUID string
	for _, lpar := range lpars {
		if lpar.PartitionName == *lparName {
			lparUUID = lpar.UUID
			break
		}
	}
	if lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found in system '%s'", *lparName, *systemName)
	}
	fmt.Printf("✅ Found LPAR UUID: %s\n", lparUUID)

	// 4. Get all Virtual Switches and find the target switch
	fmt.Printf("\n🔍 Looking up Virtual Switch: %s...\n", *vswitchName)
	vswitches, err := client.GetVirtualSwitchQuickAll(sysUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get Virtual Switches: %v", err)
	}

	var vswitchUUID string
	for _, vswitch := range vswitches {
		if vswitch.SwitchName == *vswitchName {
			vswitchUUID = vswitch.UUID
			break
		}
	}
	if vswitchUUID == "" {
		log.Fatalf("❌ Virtual Switch '%s' not found in system '%s'", *vswitchName, *systemName)
	}
	fmt.Printf("✅ Found Virtual Switch UUID: %s\n", vswitchUUID)

	// 5. Create the Client Network Adapter
	fmt.Printf("\n🚀 Creating Virtual Ethernet Adapter...\n")
	fmt.Printf("   System: %s\n", *systemName)
	fmt.Printf("   LPAR: %s\n", *lparName)
	fmt.Printf("   Virtual Switch: %s\n", *vswitchName)
	fmt.Printf("   VLAN ID: %d\n", *vlanID)
	
	adapter, err := client.CreateClientNetworkAdapter(sysUUID, lparUUID, vswitchUUID, *vlanID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to create adapter: %v", err)
	}

	fmt.Printf("\n✨ Successfully provisioned Virtual Ethernet Adapter!\n")
	fmt.Printf("   Adapter UUID: %s\n", adapter.UUID)
	fmt.Printf("   MAC Address: %s\n", hmc.FormatMACAddress(adapter.MACAddress))
	fmt.Printf("   VLAN ID: %s\n", adapter.PortVLANID)
	fmt.Printf("   Virtual Slot Number: %s\n", adapter.VirtualSlotNumber)
	fmt.Printf("   Location Code: %s\n", adapter.LocationCode)
	fmt.Printf("   Virtual Switch Name: %s\n", adapter.VirtualSwitchName)
	fmt.Printf("   Virtual Switch ID: %s\n", adapter.VirtualSwitchID)
	fmt.Printf("   DRC Name: %s\n", adapter.DynamicReconfigurationConnectorName)
	fmt.Printf("   Required Adapter: %s\n", adapter.RequiredAdapter)
	fmt.Printf("   Varied On: %s\n", adapter.VariedOn)
}

// Made with Bob
