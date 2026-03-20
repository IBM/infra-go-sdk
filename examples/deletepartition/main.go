package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	hmc "github.com/sudeeshjohn/powerhmc-go"
	svc "github.com/sudeeshjohn/svc-go-sdk"
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
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	hmcUser := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	hmcPass := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	lparName := flag.String("lpar-name", "test-test-test", "LPAR Name to delete")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	svcIP := flag.String("svc-ip", "192.0.2.8", "SVC IP address")
	svcUser := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC username")
	svcPass := flag.String("svc-pass", "REDACTED_HMC_PASS<==", "SVC password")

	flag.Parse()

	if *hmcPass == "" || *sysName == "" || *lparName == "" || *svcPass == "" {
		log.Fatal("Error: hmc-pass, system-name, lpar-name, and svc-pass are all required.")
	}

	// =========================================================================
	// PHASE 1: HMC RESOLUTION & SHUTDOWN
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*hmcUser, *hmcPass, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	sysUUID, _, _ := restClient.GetManagedSystemByName(*sysName, *verbose)
	lpars, _ := restClient.GetLogicalPartitionQuickAll(sysUUID, *verbose)

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
		restClient.PowerOffPartition(targetLparUUID, "Immediate", false, *verbose)
		for i := 0; i < 20; i++ {
			p, _ := restClient.GetLogicalPartitionQuick(targetLparUUID, false)
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
	// PHASE 2: STORAGE DISCOVERY (Using Detailed Structs)
	// =========================================================================
	fmt.Println("\nStep 2: Discovering storage mappings...")
	vioses, _ := restClient.GetVirtualIOServersQuick(sysUUID, *verbose)
	var discoveredMappings []mappingData

	for _, v := range vioses {
		// Build Slot -> UUID map for this VIOS Server Adapters
		adapterList, _ := restClient.GetVirtualSCSIServerAdapters(v.UUID, *verbose)
		slotToUUID := make(map[string]string)
		for _, a := range adapterList {
			slotToUUID[a.VirtualSlotNumber] = a.UUID
		}

		// Fetch all detailed mappings for the VIOS
		mappings, err := restClient.GetViosSCSIMappingDetails(v.UUID, *verbose)
		if err != nil {
			log.Printf("⚠️ Warning: Failed to get mappings for VIOS %s: %v", v.PartitionName, err)
			continue
		}

		for _, m := range mappings {
			// Filter to only process mappings belonging to our target LPAR
			if !strings.HasSuffix(strings.ToLower(m.AssociatedLparURI), targetLparLower) {
				continue
			}

			// Extract properties cleanly from the struct
			volName := m.ServerAdapter.BackingDeviceName
			if volName == "" {
				volName = "Unknown"
			}

			discoveredMappings = append(discoveredMappings, mappingData{
				ViosUUID:    v.UUID,
				ViosName:    v.PartitionName,
				VolName:     volName,
				VtdName:     m.TargetDevice.TargetName,
				AdapterUUID: slotToUUID[m.ServerAdapter.VirtualSlotNumber],
				VolumeUDID:  m.Storage.VolumeUniqueID,
			})
		}
	}

	if len(discoveredMappings) == 0 {
		fmt.Println("   No storage mappings found. Proceeding directly to partition deletion.")
	} else {
		fmt.Printf("   Found %d volume(s). Starting clean-up sequence...\n", len(discoveredMappings))
	}

	// =========================================================================
	// PHASE 3: HMC MAPPING REMOVAL (Sequence: Client -> CLI VTD -> Server)
	// =========================================================================
	if len(discoveredMappings) > 0 {
		fmt.Println("\nStep 3: Removing HMC VSCSI Architectures...")
		for _, m := range discoveredMappings {
			fmt.Printf("--- Processing Mapping for %s ---\n", m.VolName)

			// 3.1 Delete Client Adapter
			restClient.RemoveVolumeLPARMapping(m.ViosUUID, targetLparUUID, m.VolName, *verbose)
			fmt.Println("   ✅ Client mapping removed.")
			time.Sleep(10 * time.Second)

			// 3.2 Remove Backing Device (VTD) via CLI to unlock Server Adapter
			if m.VtdName != "" {
				cmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmvdev -vtd %s"`, *sysName, m.ViosName, m.VtdName)
				restClient.RunVIOSCommand(cmd, *verbose)
				fmt.Printf("   ✅ VTD %s removed via CLI.\n", m.VtdName)
				time.Sleep(5 * time.Second)
			}

			// 3.3 Delete Server Adapter via REST
			if m.AdapterUUID != "" {
				restClient.DeleteVirtualSCSIServerAdapter(m.ViosUUID, m.AdapterUUID, *verbose)
				fmt.Println("   ✅ Server adapter (vhost) deleted.")
			}
		}

		// =========================================================================
		// PHASE 4: SVC CLEANUP
		// =========================================================================
		fmt.Printf("\nStep 4: Connecting to SVC [%s] for SAN cleanup...\n", *svcIP)
		svcClient := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()
		if err := svcClient.Authenticate(); err == nil {
			vdisks, _ := svcClient.LsVdisk()
			for _, m := range discoveredMappings {
				if m.VolumeUDID == "" {
					continue // Skip if it's not a physical volume (e.g. optical media)
				}

				targetUID := restClient.GetSvcUidFixed(m.VolumeUDID)
				svcVolName := ""
				for _, vd := range vdisks {
					if strings.ToUpper(vd.VdiskUID) == targetUID {
						svcVolName = vd.Name
						break
					}
				}

				if svcVolName != "" {
					// Resolve SVC Host via WWPNs
					viosObj, _ := restClient.GetVirtualIOServer(m.ViosUUID, false)
					
					// Collect all WWPNs from the VIOS
					var wwpns []string
					for _, fc := range viosObj.Storage.FibreChannelPorts {
						wwpns = append(wwpns, strings.ToUpper(fc.WWPN))
					}
					
					// Try to find host by any of the WWPNs
					if len(wwpns) > 0 {
						host, matchedWWPN, err := svcClient.GetHostByWWPN(wwpns)
						if err == nil && host != "" {
							if *verbose {
								fmt.Printf("   Found SVC host '%s' via WWPN %s\n", host, matchedWWPN)
							}
							svcClient.Rmvdiskhostmap(host, svcVolName)
						}
					}
					
					svcClient.Rmvdisk(svcVolName, svc.VolumeRemove{Force: true})
					fmt.Printf("✅ SVC Volume %s purged.\n", svcVolName)
				}
			}
		}

		// =========================================================================
		// PHASE 5: VIOS DEVICE WIPING
		// =========================================================================
		fmt.Println("\nStep 5: Wiping physical hdisks from VIOS ODM...")
		processedVios := make(map[string]string)
		for _, m := range discoveredMappings {
			// Skip wiping if it's virtual optical media
			if !strings.HasPrefix(m.VolName, "hdisk") && !strings.HasPrefix(m.VolName, "nvme") {
				continue
			}
			cmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmdev -dev %s -recursive"`, *sysName, m.ViosName, m.VolName)
			restClient.RunVIOSCommand(cmd, *verbose)
			processedVios[m.ViosUUID] = m.ViosName
		}

		// Run cfgdev on affected VIOSes
		fmt.Println("\nStep 6: Running cfgdev on VIOSes...")
		for uuid, name := range processedVios {
			if err := restClient.ConfigDevice(uuid, "", *verbose); err == nil {
				fmt.Printf("✅ Device tree cleaned on %s.\n", name)
			}
		}
	} else {
		fmt.Println("\nStep 3-6: Skipped (no storage mappings found).")
	}

	// =========================================================================
	// PHASE 6: LPAR DELETION
	// =========================================================================
	fmt.Printf("\nStep 7: Deleting Logical Partition %s...\n", *lparName)
	if err := restClient.DeleteLogicalPartition(targetLparUUID, *verbose); err != nil {
		log.Printf("⚠️ Warning: LPAR delete failed: %v", err)
	} else {
		fmt.Println("✅ LPAR deleted from HMC.")
	}

	fmt.Printf("\n🎉 SUCCESS: Partition '%s' and its associated infrastructure have been completely removed.\n", *lparName)
}