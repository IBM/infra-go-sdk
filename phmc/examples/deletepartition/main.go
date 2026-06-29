package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	svc "github.com/IBM/infra-go-sdk/svc"
	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

type mappingData struct {
	ViosUUID    string
	ViosName    string
	VolName     string
	VtdName     string
	AdapterUUID string
	VolumeUDID  string
}

func main() {
	// =========================================================================
	// CONFIGURATION - Command Line Flags
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	hmcUser := flag.String("hmc-user", "", "HMC username")
	hmcPass := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	lparName := flag.String("lpar-name", "sno-master", "LPAR Name to delete")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	svcIP := flag.String("svc-ip", "", "SVC IP address")
	svcUser := flag.String("svc-user", "", "SVC username")
	svcPass := flag.String("svc-pass", "", "SVC password")

	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel() // Automatically cleans up the timer/goroutine the second the function exits

	if *hmcPass == "" || *sysName == "" || *lparName == "" || *svcPass == "" {
		log.Fatal("Error: hmc-pass, system-name, lpar-name, and svc-pass are all required.")
	}

	// =========================================================================
	// PHASE 1: HMC RESOLUTION & SHUTDOWN
	// =========================================================================
	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)
	if err := restClient.Login(context.Background(), *hmcUser, *hmcPass); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	sysUUID, _, _ := restClient.GetManagedSystemByName(context.Background(), *sysName)
	lpars, _ := restClient.GetLogicalPartitionsQuickAll(context.Background(), sysUUID)

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
		log.Fatalf("LPAR %s not found on system %s", *lparName, *sysName)
	}

	targetLparLower := strings.ToLower(targetLparUUID)

	// Shutdown Partition
	if currentState != "not activated" {
		fmt.Printf("Step 1: Partition is '%s'. Powering off...\n", currentState)
		restClient.PowerOffPartition(ctx,targetLparUUID, "Immediate", false)
		for i := 0; i < 20; i++ {
			p, _ := restClient.GetLogicalPartitionQuick(targetLparUUID)
			if p != nil && strings.ToLower(p.PartitionState) == "not activated" {
				break
			}
			time.Sleep(5 * time.Second)
		}
		fmt.Println("✅ Partition powered off.")
	} else {
		fmt.Println("Step 1: Partition is already 'not activated'. Skipping shutdown.")
	}

	// =========================================================================
	// PHASE 2: STORAGE DISCOVERY (Using GetViosSCSIMappings)
	// =========================================================================
	fmt.Println("\nStep 2: Discovering storage mappings...")
	vioses, _ := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID)
	var discoveredMappings []mappingData
	
	// Storage categorization for batch deletion
	type viosStorage struct {
		physicalVols []string
		virtualDisks []string
		opticalMedia []string
	}
	viosStorageMap := make(map[string]*viosStorage)
	viosNames := make(map[string]string)

	for _, v := range vioses {
		viosNames[v.UUID] = v.PartitionName
		viosStorageMap[v.UUID] = &viosStorage{
			physicalVols: []string{},
			virtualDisks: []string{},
			opticalMedia: []string{},
		}
		
		// Fetch all detailed mappings for the VIOS
		mappings, err := restClient.GetViosSCSIMappings(context.Background(), v.UUID)
		if err != nil {
			log.Printf("⚠️ Warning: Failed to get mappings for VIOS %s: %v", v.PartitionName, err)
			continue
		}

		for _, m := range mappings {
			// Filter to only process mappings belonging to our target LPAR
			if !strings.HasSuffix(strings.ToLower(m.AssociatedLogicalPartition.Href), targetLparLower) {
				continue
			}

			// Extract volume name and determine storage type
			volName := m.ServerAdapter.BackingDeviceName
			if volName == "" {
				volName = "Unknown"
			}

			// Determine storage type based on which field is populated
			storageType := ""
			volumeUDID := ""
			
			if m.Storage.PhysicalVolume.VolumeName != "" {
				storageType = "PhysicalVolume"
				volumeUDID = m.Storage.PhysicalVolume.VolumeUniqueID
			} else if m.Storage.VirtualDisk.DiskName != "" {
				storageType = "VirtualDisk"
				volumeUDID = m.Storage.VirtualDisk.UniqueDeviceID
			} else if m.Storage.VirtualOpticalMedia.MediaName != "" {
				storageType = "VirtualOpticalMedia"
				volumeUDID = m.Storage.VirtualOpticalMedia.MediaUDID
			} else {
				// Fallback: Infer from TargetDevice type
				if m.TargetDevice.LogicalVolumeVirtualTargetDevice.TargetName != "" {
					storageType = "VirtualDisk"
				} else if m.TargetDevice.PhysicalVolumeVirtualTargetDevice.TargetName != "" {
					storageType = "PhysicalVolume"
				} else if m.TargetDevice.VirtualOpticalTargetDevice.TargetName != "" {
					storageType = "VirtualOpticalMedia"
				}
			}
			
			switch storageType {
			case "PhysicalVolume":
				viosStorageMap[v.UUID].physicalVols = append(viosStorageMap[v.UUID].physicalVols, volName)
			case "VirtualDisk":
				viosStorageMap[v.UUID].virtualDisks = append(viosStorageMap[v.UUID].virtualDisks, volName)
			case "VirtualOpticalMedia":
				viosStorageMap[v.UUID].opticalMedia = append(viosStorageMap[v.UUID].opticalMedia, volName)
			}

			// Store for SVC cleanup and adapter deletion
			discoveredMappings = append(discoveredMappings, mappingData{
				ViosUUID:    v.UUID,
				ViosName:    v.PartitionName,
				VolName:     volName,
				AdapterUUID: m.ServerAdapter.UniqueDeviceID,
				VolumeUDID:  volumeUDID,
			})
		}
	}

	if len(discoveredMappings) == 0 {
		fmt.Println("   No storage mappings found. Proceeding directly to partition deletion.")
	} else {
		fmt.Printf("   Found %d volume(s). Starting clean-up sequence...\n", len(discoveredMappings))
	}

	// =========================================================================
	// PHASE 3: HMC MAPPING REMOVAL (Using Batch Delete Functions)
	// =========================================================================
	if len(discoveredMappings) > 0 {
		fmt.Println("\nStep 3: Removing HMC Storage Mappings...")
		
		for viosUUID, storage := range viosStorageMap {
			viosName := viosNames[viosUUID]
			
			// Delete Physical Volumes
			if len(storage.physicalVols) > 0 {
				fmt.Printf("   VIOS %s: Unmapping physical volumes: %s\n", viosName, strings.Join(storage.physicalVols, ", "))
				result, err := restClient.DeletePhysicalVolumeMaps(sysUUID, viosUUID, targetLparUUID, storage.physicalVols)
				if err != nil {
					log.Printf("⚠️ Warning: Failed to delete physical volume mappings: %v", err)
				} else {
					fmt.Printf("   ✅ Physical volumes unmapped: %s\n", result)
				}
			}
			
			// Delete Virtual Disks
			if len(storage.virtualDisks) > 0 {
				fmt.Printf("   VIOS %s: Unmapping virtual disks: %s\n", viosName, strings.Join(storage.virtualDisks, ", "))
				result, err := restClient.DeleteVirtualDiskMaps(sysUUID, viosUUID, targetLparUUID, storage.virtualDisks)
				if err != nil {
					log.Printf("⚠️ Warning: Failed to delete virtual disk mappings: %v", err)
				} else {
					fmt.Printf("   ✅ Virtual disks unmapped: %s\n", result)
					
					// After successful unmapping, delete the virtual disks themselves
					fmt.Printf("   VIOS %s: Deleting virtual disks from storage pool...\n", viosName)
					for _, diskName := range storage.virtualDisks {
						if err := restClient.DeleteVirtualDisk(context.Background(), *sysName, viosName, diskName); err != nil {
							log.Printf("⚠️ Warning: Failed to delete virtual disk '%s': %v", diskName, err)
						} else {
							fmt.Printf("   🗑️  Virtual disk '%s' deleted successfully\n", diskName)
						}
					}
				}
			}
			
			// Delete Optical Media
			if len(storage.opticalMedia) > 0 {
				fmt.Printf("   VIOS %s: Unmapping optical media: %s\n", viosName, strings.Join(storage.opticalMedia, ", "))
				result, err := restClient.DeleteVirtualOpticalMaps(context.Background(), sysUUID, viosUUID, targetLparUUID, storage.opticalMedia)
				if err != nil {
					log.Printf("⚠️ Warning: Failed to delete optical media mappings: %v", err)
				} else {
					fmt.Printf("   ✅ Optical media unmapped: %s\n", result)
				}
			}
		}

		// =========================================================================
		// PHASE 4: SVC CLEANUP
		// =========================================================================
		fmt.Printf("\nStep 4: Connecting to SVC [%s] for SAN cleanup...\n", *svcIP)
		svcClient := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()
		if err := svcClient.Authenticate(ctx); err == nil {
			vdisks, _ := svcClient.LsVdisk(ctx)
			for _, m := range discoveredMappings {
				if m.VolumeUDID == "" {
					continue // Skip if it's not a physical volume (e.g. optical media)
				}

				targetUID := restClient.GetSvcUIDFixed(m.VolumeUDID)
				svcVolName := ""
				for _, vd := range vdisks {
					if strings.ToUpper(vd.VdiskUID) == targetUID {
						svcVolName = vd.Name
						break
					}
				}

				if svcVolName != "" {
					// Resolve SVC Host via WWPNs
					viosObj, _ := restClient.GetVirtualIOServer(context.Background(), m.ViosUUID)
					
					// Collect all WWPNs from the VIOS
					var wwpns []string
					// Extract WWPNs from nested structure
					for _, profileSlot := range viosObj.PartitionIOConfiguration.ProfileIOSlots {
						fcAdapter := profileSlot.AssociatedIOSlot.RelatedIOAdapter.PhysicalFibreChannelAdapter
						for _, fc := range fcAdapter.PhysicalFibreChannelPorts {
							wwpns = append(wwpns, strings.ToUpper(fc.WWPN))
						}
					}
					
					// Try to find host by any of the WWPNs
					if len(wwpns) > 0 {
						host, matchedWWPN, err := svcClient.GetHostByWWPN(ctx, wwpns)
						if err == nil && host != "" {
							if *verbose {
								fmt.Printf("   Found SVC host '%s' via WWPN %s\n", host, matchedWWPN)
							}
							svcClient.Rmvdiskhostmap(ctx, host, svcVolName)
						}
					}
					
					svcClient.Rmvdisk(ctx, svcVolName, svc.VolumeRemove{Force: true})
					fmt.Printf("✅ SVC Volume %s purged.\n", svcVolName)
				}
			}
		}

		// =========================================================================
		// PHASE 6: VIOS DEVICE WIPING
		// =========================================================================
		fmt.Println("\nStep 6: Wiping physical hdisks from VIOS ODM...")
		processedVios := make(map[string]string)
		for _, m := range discoveredMappings {
			// Skip wiping if it's virtual optical media
			if !strings.HasPrefix(m.VolName, "hdisk") && !strings.HasPrefix(m.VolName, "nvme") {
				continue
			}
			cmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmdev -dev %s -recursive"`, *sysName, m.ViosName, m.VolName)
			restClient.CliRunner(context.Background(), cmd)
			processedVios[m.ViosUUID] = m.ViosName
		}

		// Run cfgdev on affected VIOSes
		fmt.Println("\nStep 7: Running cfgdev on VIOSes...")
		for uuid, name := range processedVios {
			if err := restClient.ConfigDevice(ctx,uuid, ""); err == nil {
				fmt.Printf("✅ Device tree cleaned on %s.\n", name)
			}
		}
	} else {
		fmt.Println("\nStep 3-6: Skipped (no storage mappings found).")
	}

	// =========================================================================
	// PHASE 5: LPAR DELETION
	// =========================================================================
	fmt.Printf("\nStep 5: Deleting Logical Partition %s...\n", *lparName)
	if err := restClient.DeleteLogicalPartition(targetLparUUID); err != nil {
		log.Printf("⚠️ Warning: LPAR delete failed: %v", err)
	} else {
		fmt.Println("✅ LPAR deleted from HMC.")
	}

	fmt.Printf("\n🎉 SUCCESS: Partition '%s' and its associated infrastructure have been completely removed.\n", *lparName)
}
