package main

import (
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/PowerHMC" // Adjust to your package path
)

func main() {
	// =========================================================================
	// CONFIGURATION
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	lparName := flag.String("lpar-name", "test-test-test", "LPAR Name to search")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" {
		log.Fatal("Error: hmc-pass is required.")
	}

	// 1. Initialize & Login
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	// 2. Resolve IDs
	sysUUID, _, _ := restClient.GetManagedSystemByName(*sysName, *verbose)
	lpars, _ := restClient.GetLogicalPartitionsQuickAll(sysUUID, *verbose)
	
	var targetLparUUID string
	for _, l := range lpars {
		if l.PartitionName == *lparName {
			targetLparUUID = l.UUID
			break
		}
	}

	if targetLparUUID == "" {
		log.Fatalf("Partition %s not found on system %s", *lparName, *sysName)
	}

	// 3. Scan VIOSes for this LPAR's Mappings
	vioses, _ := restClient.GetVirtualIOServersQuick(sysUUID, *verbose)

	fmt.Printf("\n--- VSCSI Mappings for LPAR: %s (%s) ---\n", *lparName, targetLparUUID)

	for _, v := range vioses {
		// CALL THE NEW UTIL FUNCTION
		mappings, err := restClient.GetViosSCSIMapping(v.UUID, targetLparUUID, *verbose)
		if err != nil {
			log.Printf("Error checking VIOS %s: %v", v.PartitionName, err)
			continue
		}

		if len(mappings) == 0 {
			continue
		}

		fmt.Printf("\nFound on VIOS: %s\n", v.PartitionName)
		for i, mapping := range mappings {
			// Extract display data (Volume Name and Slots)
			volElem := mapping.FindElement(".//*[local-name()='VolumeName']")
			backDevElem := mapping.FindElement(".//*[local-name()='BackingDeviceName']")
			cSlot := mapping.FindElement(".//*[local-name()='ClientAdapter']/*[local-name()='VirtualSlotNumber']")
			sSlot := mapping.FindElement(".//*[local-name()='ServerAdapter']/*[local-name()='VirtualSlotNumber']")
			AdpName := mapping.FindElement(".//*[local-name()='ServerAdapter']/*[local-name()='AdapterName']")

			volName := "Unknown"
			if volElem != nil { volName = volElem.Text() } else if backDevElem != nil { volName = backDevElem.Text() }
			
			clientSlot := "N/A"
			if cSlot != nil { clientSlot = cSlot.Text() }
			
			serverSlot := "N/A"
			if sSlot != nil { serverSlot = sSlot.Text() }

			backDev := "N/A"
			if sSlot != nil { backDev = backDevElem.Text() }

			adapterName:= "N/A"
			if sSlot != nil { adapterName= AdpName.Text() }

			fmt.Printf("  [%d] Volume: %-15s | Client Slot: %-3s | Server Slot: %-3s | Backing Device: %-3s | Adaptor Name: %-3s\n", 
				i+1, volName, clientSlot, serverSlot, backDev, adapterName)
		}
	}
}
