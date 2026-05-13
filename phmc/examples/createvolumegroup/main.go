package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	
	// We no longer ask for specific disks. We just ask for the VG name and how many disks to use.
	vgName := flag.String("vg-name", "auto_vg01", "Name of the new Volume Group")
	diskCount := flag.Int("disk-count", 1, "Number of free disks to allocate to the new VG")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *vgName == "" {
		log.Fatal("Error: hmc-pass and vg-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// 1. DYNAMIC SYSTEM & VIOS DISCOVERY
	// =========================================================================
	fmt.Printf("\nResolving System Name: %s...\n", *sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System %s not found: %v", *sysName, err)
	}

	fmt.Println("Discovering Virtual I/O Servers...")
	viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID, *verbose)
	if err != nil || len(viosList) == 0 {
		log.Fatalf("❌ Failed to fetch VIOS instances for system %s.", *sysName)
	}

	// =========================================================================
	// 2. DISCOVER FREE DISKS
	// =========================================================================
	var targetViosUUID, targetViosName string
	var selectedDisks []string

	fmt.Println("\nScanning VIOS instances for available unmapped storage...")
	for _, vios := range viosList {
		// Use the SDK to find physical volumes that are NOT part of a VG and NOT mapped to an LPAR
		freeDisks, err := restClient.GetFreePhyVolume(vios.UUID, *verbose)
		if err != nil {
			continue // Skip if there's an error querying this specific VIOS
		}

		fmt.Printf("   -> VIOS '%s' has %d free disk(s) available.\n", vios.PartitionName, len(freeDisks))

		// If this VIOS has enough free disks to satisfy our requirement, lock it in!
		if len(freeDisks) >= *diskCount {
			targetViosUUID = vios.UUID
			targetViosName = vios.PartitionName

			// Grab the exact number of disks requested
			for i := 0; i < *diskCount; i++ {
				selectedDisks = append(selectedDisks, freeDisks[i].VolumeName)
				fmt.Printf("      - Selecting: %s (Capacity: %d MB, UID: %s)\n",
					freeDisks[i].VolumeName, freeDisks[i].VolumeCapacity, freeDisks[i].UniqueDeviceID)
			}
			break // We found what we needed, no need to check the other VIOSes
		}
	}

	// Abort if we couldn't find enough free disks anywhere
	if targetViosUUID == "" {
		log.Fatalf("❌ Could not find a VIOS with at least %d free disk(s).", *diskCount)
	}

	// =========================================================================
	// 3. EXECUTE VOLUME GROUP CREATION
	// =========================================================================
	fmt.Printf("\n🚀 Creating Volume Group '%s' on VIOS '%s'...\n", *vgName, targetViosName)
	fmt.Printf("   -> Disks being assigned: %v\n", selectedDisks)

	// Note: If you get an HTTP 405 error, remember to change "PUT" to "POST" in the CreateVolumeGroup SDK function.
	err = restClient.CreateVolumeGroup(targetViosUUID, *vgName, selectedDisks, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to create Volume Group: %v", err)
	}

	fmt.Printf("\n🎉 Successfully created Volume Group '%s' using dynamically discovered disks!\n", *vgName)
}