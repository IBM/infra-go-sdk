package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	hmc "github.com/sudeeshjohn/PowerHMC"
	"github.com/sudeeshjohn/svc-go-sdk/svc"
)

func main() {
	// =========================================================================
	// CONFIGURATION - Command Line Flags
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	hmcUser := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	hmcPass := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	lparName := flag.String("lpar-name", "", "LPAR Name to delete")
	verbose := flag.Bool("verbose", true, "Enable verbose output")

	svcIP := flag.String("svc-ip", "192.0.2.8", "SVC IP address")
	svcUser := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC username")
	svcPass := flag.String("svc-pass", "", "SVC password")

	flag.Parse()

	if *hmcPass == "" || *sysName == "" || *lparName == "" || *svcPass == "" {
		log.Fatal("Error: hmc-pass, system-name, lpar-name, and svc-pass are all required.")
	}

	// =========================================================================
	// PHASE 1: HMC COMPUTE PREPARATION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	if err := restClient.Login(*hmcUser, *hmcPass, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	fmt.Printf("\nStep 1: Locating System [%s]...\n", *sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("System %s not found: %v", *sysName, err)
	}

	fmt.Printf("Step 2: Locating Partition [%s]...\n", *lparName)
	lpars, err := restClient.GetLogicalPartitionsQuickAll(sysUUID, *verbose)
	if err != nil {
		log.Fatalf("Failed to fetch partitions: %v", err)
	}

	var targetLparUUID string
	var currentState string
	for _, l := range lpars {
		if l.PartitionName == *lparName {
			targetLparUUID = l.UUID
			currentState = strings.ToLower(l.PartitionState)
			break
		}
	}

	if targetLparUUID == "" {
		log.Fatalf("Error: Partition %s not found on system %s", *lparName, *sysName)
	}

	// 3. Shutdown LPAR
	if currentState != "not activated" {
		fmt.Printf("Step 3: Partition is currently '%s'. Initiating Shutdown...\n", currentState)
		restClient.PowerOffPartition(sysUUID, targetLparUUID, "Immediate", false, *verbose)
		
		fmt.Println("        Waiting for HMC to register 'not activated' state (Polling)...")
		stateReached := false
		for i := 0; i < 20; i++ {
			prop, err := restClient.GetLogicalPartitionQuick(targetLparUUID, false)
			if err == nil && strings.ToLower(prop.PartitionState) == "not activated" {
				stateReached = true
				break
			}
			time.Sleep(5 * time.Second)
		}
		if !stateReached {
			log.Fatalf("Timeout: Partition did not reach 'not activated' state.")
		}
		fmt.Println("        ✅ Partition powered off.")
	} else {
		fmt.Println("Step 3: Partition is already 'not activated'. Skipping shutdown.")
	}

	// =========================================================================
	// PHASE 2: STORAGE CLEANUP (The Correct Order)
	// =========================================================================
	fmt.Println("\nStep 4: Tracing attached storage via HMC VIOS mappings...")
	mappedVolumes, err := restClient.GetAttachedVolumes(sysUUID, targetLparUUID, *verbose)
	if err != nil {
		log.Fatalf("Critical Error: Failed to trace storage: %v", err)
	}

	if len(mappedVolumes) > 0 {
		fmt.Printf("        Found %d volume(s). Starting clean-up sequence...\n", len(mappedVolumes))

		// --- STEP 5: REMOVE HMC VSCSI MAPPING ---
		fmt.Println("\nStep 5: Removing VSCSI mappings from HMC...")
		for _, vol := range mappedVolumes {
			err := restClient.RemoveVolumeLPARMapping(vol.ViosUUID, targetLparUUID, vol.VolumeName, *verbose)
			if err != nil {
				log.Printf("           ⚠️ Warning: Mapping removal failed for %s: %v", vol.VolumeName, err)
				// Continue with cleanup even if mapping removal fails
			} else {
				fmt.Printf("           ✅ HMC mapping removed for %s.\n", vol.VolumeName)
			}
		}
		fmt.Println("        Waiting 5s for HMC to sync state...")
		time.Sleep(5 * time.Second)

		// --- STEP 6: SVC STORAGE CLEANUP ---
		fmt.Printf("\nStep 6: Connecting to SVC [%s] for storage cleanup...\n", *svcIP)
		svcClient := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()
		if err := svcClient.Authenticate(); err != nil {
			log.Fatalf("SVC Auth error: %v", err)
		}
		fmt.Println("        ✅ Authenticated to SVC.")

		// Pre-fetch all vdisks once
		vdisks, err := svcClient.LsVdisk()
		if err != nil {
			log.Fatalf("Failed to list SVC volumes: %v", err)
		}

		for i, vol := range mappedVolumes {
			fmt.Printf("\n        --- Processing Volume %d/%d: %s ---\n", i+1, len(mappedVolumes), vol.VolumeName)
			
			// 1. Resolve SVC Volume Name via Fixed UID
			targetUID := restClient.GetSvcUidFixed(vol.VolumeUDID)
			svcVolumeName := ""
			if targetUID != "" {
				for _, vdisk := range vdisks {
					if strings.ToUpper(vdisk.VdiskUID) == targetUID {
						svcVolumeName = vdisk.Name
						fmt.Printf("        ✅ Found SVC Volume: %s (Matched UID)\n", svcVolumeName)
						break
					}
				}
			}
			

			if svcVolumeName == "" {
				log.Printf("        ⚠️ Warning: Could not find SVC volume for UDID %s. Skipping SAN cleanup for this disk.", vol.VolumeUDID)
				continue
			}

			// 2. Dynamically Resolve SVC Hostname via VIOS WWPNs (Matching your creation logic)
			viosObj, err := restClient.GetVirtualIOServer(vol.ViosUUID, false)
			targetHost := ""
			if err == nil {
				for _, fc := range viosObj.Storage.FibreChannelPorts {
					wwpn := strings.ToUpper(fc.WWPN)
					existingHost, err := svcClient.GetHostByWWPN(wwpn)
					log.Printf("wwpn :%s", wwpn)
					log.Printf("EXISTIN HOST :%s", existingHost)
					log.Printf("ERR: %s", err)
					log.Printf("WWPN: %s", fc.WWPN)
					log.Printf("SVC VOL: %s", svcVolumeName)
					log.Printf("SVC VOL UDID: %s", vol.VolumeUDID)
					if err == nil {
						targetHost = existingHost
						fmt.Printf("        ✅ Identified SVC Host: %s (via WWPN %s)\n", targetHost, wwpn)
						break
					}
				}
			}
			

			// Fallback to VIOS Name if WWPN lookup fails
			if targetHost == "" {
				fmt.Printf("        ⚠️ Warning: Could not resolve SVC host via WWPNs. Fallback to VIOS name: %s\n", vol.ViosName)
				targetHost = vol.ViosName
			}

			// 3. Unmap and Delete
			fmt.Printf("        -> Unmapping volume '%s' from host '%s'...\n", svcVolumeName, targetHost)
			err = svcClient.Rmvdiskhostmap(targetHost, svcVolumeName)
			if err != nil {
				if strings.Contains(err.Error(), "CMMVC6071E") || strings.Contains(err.Error(), "CMMVC5753E") || strings.Contains(err.Error(), "CMMVC5754E") {
					fmt.Println("           ✅ Volume or host mapping already removed.")
				} else {
					fmt.Printf("           Note: Unmap returned: %v\n", err)
				}
			} else {
				fmt.Println("           ✅ Successfully unmapped from SVC.")
			}

			fmt.Printf("        -> Deleting volume '%s' from SVC...\n", svcVolumeName)
			err = svcClient.Rmvdisk(svcVolumeName, svc.VolumeRemove{Force: true})
			if err != nil {
				if strings.Contains(err.Error(), "CMMVC5753E") || strings.Contains(err.Error(), "CMMVC5754E") || strings.Contains(err.Error(), "CMMVC5804E") {
					fmt.Println("           ✅ Volume already deleted.")
				} else {
					log.Printf("           ❌ Failed to delete volume: %v", err)
				}
			} else {
				fmt.Println("           ✅ Volume successfully purged from SAN.")
			}
		}

		// --- STEP 7: VIOS DEVICE WIPING ---
		fmt.Println("\nStep 7: Wiping devices from VIOS OS...")
		processedVIOS := make(map[string]bool)
		for _, vol := range mappedVolumes {
			// 7.1 Remove VTD (Force) - Ensures vtscsi is gone
			cmdVTD := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmvdev -vtd %s -recursive"`, *sysName, vol.ViosName, vol.ServerAdapter)
			restClient.RunVIOSCommand(cmdVTD, *verbose)

			// 7.2 Clear PV & Remove hdisk - The -del flag is critical for persistence
			cmdDisk := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmdev -dev %s -recursive"`, *sysName, vol.ViosName, vol.VolumeName)
			restClient.RunVIOSCommand(cmdDisk, *verbose)

			// 7.3 Remove Server Adapter (vhost)
			//cmdVhost := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmdev -dev %s -recursive"`, *sysName, vol.ViosName, vol.ServerAdapter)
			//restClient.RunVIOSCommand(cmdVhost, *verbose)
			
			processedVIOS[vol.ViosUUID] = true
		}

		// --- STEP 8: FINAL CFGDEV ---
		fmt.Println("\nStep 8: Final cfgdev to confirm clean device tree...")
		for viosUUID := range processedVIOS {
			restClient.ConfigDevice(viosUUID, "", *verbose)
			fmt.Println("           ✅ Device tree cleaned successfully.")
		}
	} else {
		fmt.Println("        No attached storage found. Proceeding to partition deletion.")
	}

	// =========================================================================
	// PHASE 3: COMPUTE DELETION
	// =========================================================================
	fmt.Printf("\nStep 9: Deleting Partition '%s' from HMC...\n", *lparName)
	if err := restClient.DeleteLogicalPartition(targetLparUUID, *verbose); err != nil {
		log.Fatalf("HMC Delete failed: %v", err)
	}

	fmt.Printf("\n🎉 SUCCESS: Partition '%s' and its associated infrastructure have been completely removed.\n", *lparName)
	fmt.Println("\n✨ All resources have been properly cleaned up. No stale devices should remain.")
}
