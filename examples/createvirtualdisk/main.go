package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	
	viosName := flag.String("vios-name", "", "Target VIOS (If empty, scans all VIOSes)")
	vgName := flag.String("vg-name", "", "Target Volume Group (If empty, safely auto-selects the best VG)")
	
	diskName := flag.String("disk-name", "auto_lv01", "Name of the new Virtual Disk (LV)")
	diskSize := flag.Int("disk-size", 10240, "Size of the Virtual Disk in Megabytes")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *diskName == "" || *diskSize <= 0 {
		log.Fatal("Error: hmc-pass, disk-name, and a valid disk-size (>0) are required.")
	}

	// Calculate requested size in GB for accurate capacity checking against the API
	requiredGB := float64(*diskSize) / 1024.0

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	// =========================================================================
	// 1. SYSTEM DISCOVERY
	// =========================================================================
	fmt.Printf("\nResolving System Name: %s...\n", *sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found: %v", *sysName, err)
	}

	viosList, err := restClient.GetVirtualIOServersQuick(sysUUID, *verbose)
	if err != nil || len(viosList) == 0 {
		log.Fatalf("❌ Failed to fetch VIOS instances for system '%s'.", *sysName)
	}

	// =========================================================================
	// 2. SMART CAPACITY DISCOVERY (Find the best place for the disk)
	// =========================================================================
	var targetViosUUID, targetViosName, targetVgName string
	var usingRootVgFallback bool

	fmt.Printf("\nScanning for a Volume Group with at least %.2f GB of free space...\n", requiredGB)

	for _, vios := range viosList {
		if *viosName != "" && !strings.EqualFold(vios.PartitionName, *viosName) {
			continue
		}

		vgList, err := restClient.GetVolumeGroups(vios.UUID, *verbose)
		if err != nil {
			continue 
		}

		for _, vg := range vgList {
			if *vgName != "" && !strings.EqualFold(vg.GroupName, *vgName) {
				continue
			}

			freeSpaceGB, parseErr := strconv.ParseFloat(vg.FreeSpace, 64)
			if parseErr != nil {
				continue
			}

			fmt.Printf("   -> Checked VIOS '%s', VG '%s': %.2f GB Free\n", vios.PartitionName, vg.GroupName, freeSpaceGB)

			if freeSpaceGB >= requiredGB {
				if *vgName == "" {
					// Smart selection: Avoid rootvg if possible
					if strings.ToLower(vg.GroupName) == "rootvg" {
						if targetVgName == "" {
							targetViosUUID = vios.UUID
							targetViosName = vios.PartitionName
							targetVgName = vg.GroupName
							usingRootVgFallback = true
							fmt.Println("      ⚠️ rootvg has space. Keeping as fallback, but searching for data VG...")
						}
					} else {
						targetViosUUID = vios.UUID
						targetViosName = vios.PartitionName
						targetVgName = vg.GroupName
						usingRootVgFallback = false
						fmt.Printf("      ✅ PERFECT MATCH! Selecting data VG '%s' on VIOS '%s'.\n", targetVgName, targetViosName)
						break 
					}
				} else {
					// Explicit match requested
					targetViosUUID = vios.UUID
					targetViosName = vios.PartitionName
					targetVgName = vg.GroupName
					fmt.Printf("      ✅ MATCH FOUND! Selecting requested VG '%s' on VIOS '%s'.\n", targetVgName, targetViosName)
					break 
				}
			}
		}

		if targetVgName != "" && !usingRootVgFallback {
			break
		}
	}

	if targetVgName == "" {
		if *vgName != "" {
			log.Fatalf("❌ Volume Group '%s' either does not exist or does not have %.2f GB of free space.", *vgName, requiredGB)
		} else {
			log.Fatalf("❌ System Exhaustion: Could not find any Volume Group with at least %.2f GB of free space.", requiredGB)
		}
	} else if usingRootVgFallback {
		fmt.Printf("\n   ⚠️ WARNING: No data Volume Groups had enough space. Falling back to '%s' on '%s'.\n", targetVgName, targetViosName)
	}

	// =========================================================================
	// 3. EXECUTE VIRTUAL DISK CREATION (Via Smart CLI Wrapper)
	// =========================================================================
	fmt.Printf("\n🚀 Creating Virtual Disk '%s' (%d MB) inside VG '%s'...\n", *diskName, *diskSize, targetVgName)

	// Call the SDK function with the targetViosUUID included
	err = restClient.CreateVirtualDisk(*sysName, targetViosUUID, targetViosName, targetVgName, *diskName, *diskSize, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to create Virtual Disk: %v", err)
	}

	fmt.Printf("\n🎉 Successfully created Virtual Disk '%s'!\n", *diskName)
}