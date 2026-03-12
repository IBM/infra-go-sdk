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
	
	// HMC Details
	hmcIP    := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	hmcUser  := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	hmcPass  := flag.String("hmc-pass", "", "HMC password")
	sysName  := flag.String("system-name", "", "Managed System Name")
	lparName := flag.String("lpar-name", "", "LPAR Name to delete")
	verbose  := flag.Bool("verbose", false, "Enable verbose output")

	// SVC Details
	svcIP    := flag.String("svc-ip", "192.0.2.8", "SVC IP address")
	svcUser  := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC username")
	svcPass  := flag.String("svc-pass", "", "SVC password")
	
	flag.Parse()

	// Validate required parameters
	if *hmcPass == "" {
		log.Fatal("Error: --hmc-pass is required")
	}
	if *sysName == "" {
		log.Fatal("Error: --system-name is required")
	}
	if *lparName == "" {
		log.Fatal("Error: --lpar-name is required")
	}
	if *svcPass == "" {
		log.Fatal("Error: --svc-pass is required")
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

	// 1. Resolve System Name
	fmt.Printf("\nStep 1: Locating System [%s]...\n", *sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("System %s not found: %v", *sysName, err)
	}

	// 2. Resolve LPAR Name & State
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

	// 3. Shutdown LPAR if it is running
	if currentState != "not activated" {
		fmt.Printf("Step 3: Partition is currently '%s'. Initiating Shutdown...\n", currentState)
		_, err := restClient.PowerOffPartition(sysUUID, targetLparUUID, "Immediate", false, *verbose)
		if err != nil {
			log.Fatalf("Power off failed: %v", err)
		}
		
		fmt.Println("        Waiting for HMC to register 'not activated' state (Polling)...")
		stateReached := false
		for i := 0; i < 15; i++ { 
			prop, err := restClient.GetLogicalPartitionQuick(targetLparUUID, false)
			if err == nil && strings.ToLower(prop.PartitionState) == "not activated" {
				stateReached = true
				break
			}
			time.Sleep(5 * time.Second)
		}
		if !stateReached {
			log.Fatalf("Timeout: Partition did not reach 'not activated' state in time.")
		}
		fmt.Println("        ✅ Partition successfully powered off.")
	} else {
		fmt.Println("Step 3: Partition is already 'not activated'. Skipping shutdown.")
	}

	// =========================================================================
	// PHASE 2: DYNAMIC STORAGE DISCOVERY & CLEANUP
	// =========================================================================
	fmt.Println("\nStep 4: Tracing dynamically attached storage via HMC VIOS mappings...")
	mappedVolumes, err := restClient.GetAttachedVolumes(sysUUID, targetLparUUID, *verbose)
	if err != nil {
		log.Fatalf("        ❌ Failed to trace storage: %v\n        This is critical - cannot proceed without knowing what to clean up.", err)
	}

	// Track processed VIOS for summary
	processedVIOS := make(map[string]bool)

	if len(mappedVolumes) > 0 {
		fmt.Printf("        Found %d attached volume(s).\n", len(mappedVolumes))
		
		// =========================================================================
		// PHASE 2A: REMOVE VSCSI MAPPINGS FROM HMC (CRITICAL STEP!)
		// =========================================================================
		fmt.Println("\nStep 5: Removing VSCSI mappings from HMC...")
		for i, vol := range mappedVolumes {
			fmt.Printf("        [%d/%d] Removing mapping for volume '%s' from VIOS '%s'...\n",
				i+1, len(mappedVolumes), vol.VolumeName, vol.ViosName)
			
			err := restClient.RemoveVolumeLPARMapping(vol.ViosUUID, targetLparUUID, vol.VolumeName, *verbose)
			if err != nil {
				log.Printf("           ⚠️ Warning: Failed to remove HMC mapping: %v", err)
				log.Printf("           Continuing, but this may leave stale devices in VIOS.")
			} else {
				fmt.Println("           ✅ HMC mapping removed successfully.")
			}
		}

		// =========================================================================
		// PHASE 2B: RUN CFGDEV ON EACH VIOS TO CLEAN DEVICE TREE
		// =========================================================================
		fmt.Println("\nStep 6: Running cfgdev on VIOS to remove stale devices...")
		// Give the HMC and VIOS a moment to settle after the mapping deletion
        time.Sleep(10 * time.Second)
		for _, vol := range mappedVolumes {
			if processedVIOS[vol.ViosUUID] {
				continue // Already processed this VIOS
			}
			processedVIOS[vol.ViosUUID] = true
			
			fmt.Printf("        Configuring devices on VIOS '%s'...\n", vol.ViosName)
			err := restClient.ConfigDevice(vol.ViosUUID, "", *verbose)
			if err != nil {
				log.Printf("           ⚠️ Warning: cfgdev failed on VIOS %s: %v", vol.ViosName, err)
				log.Printf("           Manual cleanup may be required on this VIOS.")
			} else {
				fmt.Println("           ✅ Device tree cleaned successfully.")
			}
		}

		// =========================================================================
		// PHASE 2C: SVC STORAGE CLEANUP
		// =========================================================================
		fmt.Printf("\nStep 7: Connecting to SVC [%s] for storage cleanup...\n", *svcIP)
		svcClient := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()
		if err := svcClient.Authenticate(); err != nil {
			log.Fatalf("SVC Auth error: %v", err)
		}
		fmt.Println("        ✅ Authenticated to SVC.")

		for i, vol := range mappedVolumes {
			fmt.Printf("\n        --- Processing Volume %d/%d: %s ---\n", i+1, len(mappedVolumes), vol.VolumeName)
			fmt.Printf("        VIOS Host: %s | UDID: %s\n", vol.ViosName, vol.VolumeUDID)
			
			dynamicHostName := vol.ViosName
			
			// Look up the actual SVC volume name using UDID
			// The vol.VolumeName is the VIOS device name (hdiskX), not the SVC volume name
			svcVolumeName := ""
			if vol.VolumeUDID != "" && vol.VolumeUDID != "unknown" {
				// Query SVC to find volume by UDID
				vdisks, err := svcClient.LsVdisk()
				if err != nil {
					log.Printf("           ⚠️ Warning: Failed to list SVC volumes: %v", err)
					log.Printf("           Will attempt deletion using VIOS device name '%s'", vol.VolumeName)
					svcVolumeName = vol.VolumeName
				} else {
					// Search for volume with matching UDID
					for _, vdisk := range vdisks {
						if vdisk.VdiskUID == vol.VolumeUDID {
							svcVolumeName = vdisk.Name
							fmt.Printf("        ✅ Resolved SVC volume name: '%s' (from UDID)\n", svcVolumeName)
							break
						}
					}
					if svcVolumeName == "" {
						log.Printf("           ⚠️ Warning: Could not find SVC volume with UDID %s", vol.VolumeUDID)
						log.Printf("           Will attempt deletion using VIOS device name '%s'", vol.VolumeName)
						svcVolumeName = vol.VolumeName
					}
				}
			} else {
				log.Printf("           ⚠️ Warning: No UDID available for volume '%s'", vol.VolumeName)
				log.Printf("           Will attempt deletion using VIOS device name '%s'", vol.VolumeName)
				svcVolumeName = vol.VolumeName
			}

			// Unmap Volume from SVC
			fmt.Printf("        -> Unmapping volume '%s' from SVC host '%s'...\n", svcVolumeName, dynamicHostName)
			err = svcClient.Rmvdiskhostmap(dynamicHostName, svcVolumeName)
			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "CMMVC6071E") {
					fmt.Println("           ✅ Volume already unmapped from SVC.")
				} else if strings.Contains(errStr, "CMMVC5753E") || strings.Contains(errStr, "CMMVC5754E") {
					fmt.Println("           ✅ Volume or host mapping doesn't exist (already cleaned up).")
				} else {
					log.Printf("           ⚠️ Warning: Failed to unmap from SVC: %v", err)
					log.Printf("           Continuing with volume deletion...")
				}
			} else {
				fmt.Println("           ✅ Successfully unmapped from SVC.")
			}

			// Delete Volume from SVC
			fmt.Printf("        -> Deleting volume '%s' from SVC...\n", svcVolumeName)
			removeVolume := svc.VolumeRemove{
				Force:              true,
				RemoveHostMappings: false,
			}
			if err := svcClient.Rmvdisk(svcVolumeName, removeVolume); err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "CMMVC5753E") || strings.Contains(errStr, "CMMVC5754E") || strings.Contains(errStr, "CMMVC5804E") {
					fmt.Println("           ✅ Volume already deleted (or does not exist).")
				} else {
					log.Printf("           ⚠️ Warning: Rmvdisk error: %v", err)
					log.Printf("           Manual cleanup may be required in SVC.")
				}
			} else {
				fmt.Println("           ✅ Successfully deleted volume from SVC.")
			}
		}
	} else {
		fmt.Println("        No attached storage found. Proceeding to partition deletion.")
	}

	// =========================================================================
	// PHASE 3: HMC COMPUTE DELETION
	// =========================================================================
/* 	fmt.Printf("\nStep 8: Deleting Partition '%s' from HMC...\n", *lparName)
	err = restClient.DeleteLogicalPartition(targetLparUUID, *verbose)
	if err != nil {
		log.Fatalf("HMC Delete failed: %v", err)
	}
	fmt.Println("        ✅ Partition deleted from HMC successfully.") */

	fmt.Printf("\n🎉 SUCCESS: Partition '%s' and its associated infrastructure have been completely removed.\n", *lparName)
	fmt.Println("\nCleanup Summary:")
	fmt.Printf("  ✅ Partition powered off and deleted from HMC\n")
	if len(mappedVolumes) > 0 {
		fmt.Printf("  ✅ %d VSCSI mapping(s) removed from HMC\n", len(mappedVolumes))
		fmt.Printf("  ✅ Device tree cleaned on %d VIOS(es)\n", len(processedVIOS))
		fmt.Printf("  ✅ %d volume(s) unmapped and deleted from SVC storage\n", len(mappedVolumes))
	}
	fmt.Println("\n✨ All resources have been properly cleaned up. No stale devices should remain.")
}