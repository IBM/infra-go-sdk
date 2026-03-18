package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	hmc "github.com/sudeeshjohn/PowerHMC"
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC Username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC Password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	lparName := flag.String("lpar-name", "Go_LPAR_03", "Name for the new LPAR")
	osType := flag.String("os-type", "linux", "OS type (aix, linux, aix_linux, ibmi)")

	// Networking Config
	vswitchName := flag.String("vswitch-name", "VNET0", "Name of the Virtual Switch")
	vlanID := flag.Int("vlan-id", 1, "VLAN ID for the Client Network Adapter")

	// Native Virtual Disk Config (Replaces SVC logic)
	targetVios := flag.String("vios-name", "", "Target VIOS (Leave empty for smart auto-select)")
	targetVg := flag.String("vg-name", "", "Target Volume Group (Leave empty for smart auto-select)")
	diskName := flag.String("disk-name", "lpar03_boot_lv", "Name of the Virtual Disk (LV)")
	diskSize := flag.Int("disk-size", 51200, "Size of the Virtual Disk in Megabytes")

	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	log.Println("=========================================================================")
	log.Println(" 🚀 Starting Native PowerVM Provisioning (LPAR & Virtual Disks)")
	log.Println("=========================================================================")

	// =========================================================================
	// 1. AUTHENTICATION & SYSTEM RESOLUTION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("[HMC] Logon failed: %v", err)
	}
	defer restClient.Logoff()

	sysUUID := resolveSystemUUID(restClient, *sysName, *verbose)
	ensureLparDoesNotExist(restClient, sysUUID, *lparName, *verbose)

	// =========================================================================
	// 2. PARALLEL EXECUTION WITH CHANNELS
	// =========================================================================
	lparUUIDCh := make(chan string, 1)
	lparErrCh := make(chan error, 1)
	networkErrCh := make(chan error, 1)

	type storageResult struct {
		viosUUID string
		viosName string
	}
	storageResCh := make(chan storageResult, 1)
	storageErrCh := make(chan error, 1)

	log.Println("\n🔀 Initiating Concurrent Branches (LPAR/Network vs Native Storage)...")

	// -------------------------------------------------------------------------
	// BRANCH 1: LPAR CREATION -> NETWORK ATTACHMENT
	// -------------------------------------------------------------------------
	go func() {
		log.Printf("[Branch 1] Provisioning base LPAR '%s'...", *lparName)
		req := hmc.CreateLparRequest{
			Name:             *lparName,
			MinMem:           2048,
			DesiredMem:       4096,
			MaxMem:           8192,
			MinProcUnits:     0.1,
			DesiredProcUnits: 0.5,
			MaxProcUnits:     2.0,
			MinVcpus:         1,
			DesiredVcpus:     1,
			MaxVcpus:         4,
			SharingMode:      "uncapped",
		}

		lparUUID, err := restClient.CreateLogicalPartition(sysUUID, req, *verbose)
		if err != nil {
			lparErrCh <- fmt.Errorf("LPAR Creation failed: %v", err)
			return
		}
		log.Printf("[Branch 1] ✅ LPAR Created! UUID: %s", lparUUID)
		
		// UNLOCK MAIN THREAD for Storage Mapping
		lparUUIDCh <- lparUUID 

		// Attach Network Adapter
		log.Printf("[Branch 1] Resolving Virtual Switch '%s'...", *vswitchName)
		switches, err := restClient.GetVirtualSwitchQuickAll(sysUUID, *verbose)
		if err != nil {
			networkErrCh <- fmt.Errorf("Failed to retrieve Virtual Switches: %v", err)
			return
		}

		var vswitchUUID string
		for _, s := range switches {
			if strings.EqualFold(s.SwitchName, *vswitchName) {
				vswitchUUID = s.UUID
				break
			}
		}
		if vswitchUUID == "" {
			networkErrCh <- fmt.Errorf("Virtual Switch '%s' not found", *vswitchName)
			return
		}

		log.Printf("[Branch 1] Attaching VLAN %d to LPAR...", *vlanID)
		_, err = restClient.CreateClientNetworkAdapter(sysUUID, lparUUID, vswitchUUID, *vlanID, *verbose)
		if err != nil {
			networkErrCh <- fmt.Errorf("Failed to add network adapter: %v", err)
			return
		}
		log.Printf("[Branch 1] ✅ Network Adapter Attached.")
		networkErrCh <- nil 
	}()

	// -------------------------------------------------------------------------
	// BRANCH 2: SMART CAPACITY DISCOVERY -> VIRTUAL DISK CREATION
	// -------------------------------------------------------------------------
	go func() {
		log.Printf("[Branch 2] Discovering optimal Volume Group for %d MB disk...", *diskSize)
		
		viosUUID, viosName, err := provisionVirtualDisk(restClient, *sysName, sysUUID, *diskName, *targetVios, *targetVg, *diskSize, *verbose)
		if err != nil {
			storageErrCh <- err
			return
		}
		
		log.Printf("[Branch 2] ✅ Virtual Disk '%s' Provisioned on VIOS '%s'.", *diskName, viosName)
		storageResCh <- storageResult{viosUUID: viosUUID, viosName: viosName}
	}()

	// =========================================================================
	// 3. SYNCHRONIZATION POINT 1: WAIT FOR LPAR & STORAGE
	// =========================================================================
	var finalLparUUID string
	select {
	case err := <-lparErrCh:
		log.Fatalf("❌ Branch 1 Failed: %v", err)
	case finalLparUUID = <-lparUUIDCh:
	}

	var storage storageResult
	select {
	case err := <-storageErrCh:
		log.Fatalf("❌ Branch 2 Failed: %v", err)
	case storage = <-storageResCh:
	}

	log.Println("\n=========================================================================")
	log.Println(" 🔗 Core Dependencies Met. Mapping Virtual Disk to LPAR...")
	log.Println("=========================================================================")

	// =========================================================================
	// 4. MAP VIRTUAL DISK TO LPAR
	// =========================================================================
	log.Printf("[HMC] Step 3: Attaching Virtual Disk '%s' to LPAR '%s'...", *diskName, *lparName)
	
	mappingUUID, err := restClient.CreateVirtualDiskMap(sysUUID, storage.viosUUID, finalLparUUID, *diskName, *verbose)
	if err != nil {
		log.Fatalf("[HMC] Storage Mapping Failed: %v", err)
	}
	
	if mappingUUID == "SUCCESS_WITH_RMC_WARNING" {
		log.Printf("[HMC] ✅ Disk mapped successfully! (Ignored expected RMC warning for offline LPAR)")
	} else {
		log.Printf("[HMC] ✅ Disk mapped successfully!")
	}

	// =========================================================================
	// 5. SYNCHRONIZATION POINT 2: VERIFY NETWORK & SAVE CONFIG
	// =========================================================================
	log.Println("\n[HMC] Step 4: Verifying background Network Adapter configuration...")
	if err := <-networkErrCh; err != nil {
		log.Fatalf("❌ Network Configuration Failed in background: %v", err)
	}

	log.Printf("[HMC] Step 5: Saving active configuration to profile 'default_profile'...")
	err = restClient.SaveCurrentLparConfig(finalLparUUID, "default_profile", true, *verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to save LPAR configuration: %v", err)
	}
	log.Printf("[HMC] ✅ Configuration permanently saved to profile.")

	// =========================================================================
	// 6. POWER ON THE LPAR
	// =========================================================================
	log.Printf("\n[HMC] Step 6: Powering on LPAR '%s'...", *lparName)
	
	profileUUID, err := restClient.GetPartitionProfile(finalLparUUID, *verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to get default partition profile: %v", err)
	}

	_, err = restClient.PowerOnPartition(sysUUID, finalLparUUID, profileUUID, "normal", "", *osType, *verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to PowerOn Partition: %v", err)
	}

	log.Println("\n=========================================================================")
	log.Printf(" 🎉 SUCCESS: PowerVM Provisioning Workflow Complete!")
	log.Printf("    - LPAR Name : %s is BOOTING", *lparName)
	log.Printf("    - Network   : Attached to %s (VLAN %d)", *vswitchName, *vlanID)
	log.Printf("    - Storage   : Mapped Native Virtual Disk '%s' via %s", *diskName, storage.viosName)
	log.Println("=========================================================================")
}

// =========================================================================
// WORKFLOW HELPER FUNCTIONS
// =========================================================================

func resolveSystemUUID(restClient *hmc.HmcRestClient, systemName string, verbose bool) string {
	systems, err := restClient.GetManagedSystemQuickAll(verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to get managed systems: %v", err)
	}
	for _, system := range systems {
		if strings.EqualFold(system.SystemName, systemName) {
			return system.UUID
		}
	}
	log.Fatalf("[HMC] Managed system '%s' not found.", systemName)
	return ""
}

func ensureLparDoesNotExist(restClient *hmc.HmcRestClient, systemUUID, vmName string, verbose bool) {
	existingUUID, err := restClient.GetLogicalPartitionByName(systemUUID, vmName, false)
	if err == nil && existingUUID != "" {
		log.Fatalf("[HMC] Error: LPAR with name '%s' already exists (UUID: %s)", vmName, existingUUID)
	}
}

// provisionVirtualDisk performs Smart Capacity Discovery to find the best VG, then creates the disk.
func provisionVirtualDisk(restClient *hmc.HmcRestClient, sysName, sysUUID, diskName, targetVios, targetVg string, diskSizeMB int, verbose bool) (string, string, error) {
	requiredGB := float64(diskSizeMB) / 1024.0

	viosList, err := restClient.GetVirtualIOServersQuick(sysUUID, verbose)
	if err != nil || len(viosList) == 0 {
		return "", "", fmt.Errorf("failed to fetch VIOS instances for system")
	}

	var finalViosUUID, finalViosName, finalVgName string
	var usingRootVgFallback bool

	for _, vios := range viosList {
		// Filter by VIOS if provided
		if targetVios != "" && !strings.EqualFold(vios.PartitionName, targetVios) {
			continue
		}

		vgList, err := restClient.GetVolumeGroups(vios.UUID, verbose)
		if err != nil { continue }

		for _, vg := range vgList {
			// Filter by VG if provided
			if targetVg != "" && !strings.EqualFold(vg.GroupName, targetVg) {
				continue
			}

			// Ensure no naming collision exists in this VG
			collision := false
			for _, vd := range vg.VirtualDisks {
				if strings.EqualFold(vd.DiskName, diskName) {
					collision = true
					break
				}
			}
			if collision { continue }

			freeSpaceGB, parseErr := strconv.ParseFloat(vg.FreeSpace, 64)
			if parseErr != nil { continue }

			// Capacity Check
			if freeSpaceGB >= requiredGB {
				if targetVg == "" {
					// Smart selection: Avoid rootvg if possible
					if strings.ToLower(vg.GroupName) == "rootvg" {
						if finalVgName == "" {
							finalViosUUID, finalViosName, finalVgName = vios.UUID, vios.PartitionName, vg.GroupName
							usingRootVgFallback = true
						}
					} else {
						// Found a perfect Data VG match
						finalViosUUID, finalViosName, finalVgName = vios.UUID, vios.PartitionName, vg.GroupName
						usingRootVgFallback = false
						break 
					}
				} else {
					// Explicit match requested
					finalViosUUID, finalViosName, finalVgName = vios.UUID, vios.PartitionName, vg.GroupName
					break 
				}
			}
		}
		if finalVgName != "" && !usingRootVgFallback {
			break
		}
	}

	if finalVgName == "" {
		return "", "", fmt.Errorf("could not find a Volume Group with %.2f GB of free space", requiredGB)
	}

	// Create the disk via the Smart CLI Wrapper
	err = restClient.CreateVirtualDisk(sysName, finalViosUUID, finalViosName, finalVgName, diskName, diskSizeMB, verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to create Virtual Disk via CLI: %v", err)
	}

	return finalViosUUID, finalViosName, nil
}
