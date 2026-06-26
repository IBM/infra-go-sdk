package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS")
	lparName := flag.String("lpar-name", "", "Target LPAR Name")
	lparProfile := flag.String("lpar-profile", "default_profile", "Name of the LPAR profile to overwrite")
	forceSave := flag.Bool("force-save", true, "Force overwrite of the target profile")
	
	// Default to empty string so we can safely validate it when NOT in list mode
	diskNamesStr := flag.String("disk-names", "", "Comma-separated list of Virtual Disks (LVs) to map/unmap")

	// MODE FLAGS
	deleteMode := flag.Bool("delete", false, "Set to true to DELETE the disk mappings instead of creating them")
	listMode := flag.Bool("list", false, "Set to true to purely LIST the current virtual disk mappings for the LPAR")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	
	flag.Parse()

	// --- Validation ---
	if *password == "" || *viosName == "" || *lparName == "" {
		log.Fatal("❌ Error: hmc-pass, vios-name, and lpar-name are required.")
	}

	if *deleteMode && *listMode {
		log.Fatal("❌ Error: Cannot use -delete and -list at the same time.")
	}

	if !*listMode && *diskNamesStr == "" {
		log.Fatal("❌ Error: -disk-names is required when provisioning or deleting.")
	}

	diskNames := strings.Split(*diskNamesStr, ",")
	for i := range diskNames {
		diskNames[i] = strings.TrimSpace(diskNames[i])
	}

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	viosUUID, err := hmc.GetViosID(context.Background(), restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found.", *viosName)
	}

	_, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}

	// =========================================================================
	// READ-ONLY: LIST MODE
	// =========================================================================
	if *listMode {
		fmt.Printf("\n📡 LISTING Virtual Disk Maps for LPAR '%s' on VIOS '%s'...\n", *lparName, *viosName)
		fmt.Println("=========================================================================")

		mappings, err := restClient.GetViosSCSIMappings(context.Background(), viosUUID, *verbose)
		if err != nil {
			log.Fatalf("❌ Failed to get current VIOS mappings: %v", err)
		}

		targetLparLower := strings.ToLower(lparUUID)
		count := 0

		for _, mapping := range mappings {
			if strings.HasSuffix(strings.ToLower(mapping.AssociatedLogicalPartition.Href), targetLparLower) {
				// Virtual Disks populate the VirtualDisk.DiskName field (unlike physical volumes)
				diskName := strings.TrimSpace(mapping.Storage.VirtualDisk.DiskName)
				
				if diskName != "" {
					count++
					fmt.Printf("   💾 Virtual Disk Name: %s\n", diskName)
					fmt.Printf("      - Server Adapter:  %s\n", mapping.ServerAdapter.AdapterName)
					fmt.Printf("      - Target Device:   %s\n", mapping.TargetDevice.LogicalVolumeVirtualTargetDevice.TargetName)
					fmt.Printf("      - Client Slot:     %d\n", mapping.ClientAdapter.VirtualSlotNumber)
					fmt.Printf("      - LU Address:      %s\n", mapping.TargetDevice.LogicalVolumeVirtualTargetDevice.LogicalUnitAddress)
					fmt.Printf("      - Capacity:        %s\n", mapping.Storage.VirtualDisk.DiskCapacity)
					fmt.Println("-------------------------------------------------------------------------")
				}
			}
		}

		if count == 0 {
			fmt.Printf("   ❌ No Virtual Disk mappings found for LPAR '%s' on this VIOS.\n", *lparName)
		} else {
			fmt.Printf("✅ Found %d Virtual Disk mapping(s).\n", count)
		}
		
		// Exit early for read-only mode so we don't dump XML
		return
	}

	// =========================================================================
	// DIRECTORY & FILENAME PREPARATION (For Mutations)
	// =========================================================================
	outDir := "outs"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("❌ Failed to create output directory: %v", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	safeDiskTag := strings.ReplaceAll(strings.TrimSpace(*diskNamesStr), ",", "-")
	safeDiskTag = strings.ReplaceAll(safeDiskTag, " ", "")

	beforeFile := fmt.Sprintf("%s/vios_before_%s_%s.xml", outDir, safeDiskTag, timestamp)
	afterFile := fmt.Sprintf("%s/vios_after_%s_%s.xml", outDir, safeDiskTag, timestamp)

	// =========================================================================
	// 1. DUMP "BEFORE" XML
	// =========================================================================
	fmt.Printf("\n[Diff Tool] Fetching 'BEFORE' XML state...\n")
	beforeXML, err := restClient.GetRawViosXML(viosUUID, "ViosSCSIMapping", *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch before XML: %v", err)
	}

	err = os.WriteFile(beforeFile, []byte(beforeXML), 0644)
	if err != nil {
		log.Fatalf("❌ Failed to write before file: %v", err)
	}
	fmt.Printf("   -> Saved '%s'\n", beforeFile)

	// =========================================================================
	// 2. PRE-FLIGHT CHECK: ANALYZE CURRENT MAPPINGS
	// =========================================================================
	fmt.Printf("\n[Check] Verifying current mapping state for LPAR '%s'...\n", *lparName)
	
	mappings, err := restClient.GetViosSCSIMappings(context.Background(), viosUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get current VIOS mappings: %v", err)
	}

	targetLparLower := strings.ToLower(lparUUID)
	alreadyMappedDisks := make(map[string]bool)

	for _, mapping := range mappings {
		if strings.HasSuffix(strings.ToLower(mapping.AssociatedLogicalPartition.Href), targetLparLower) {
			mappedName := strings.TrimSpace(mapping.Storage.VirtualDisk.DiskName)
			if mappedName != "" {
				alreadyMappedDisks[strings.ToLower(mappedName)] = true
			}
		}
	}

	// =========================================================================
	// 3. EXECUTE THE OPERATION (CREATE OR DELETE)
	// =========================================================================
	var operationStatus string

	if *deleteMode {
		// --- DELETE LOGIC ---
		var disksToUnmap []string
		var alreadyUnmapped []string

		for _, reqDisk := range diskNames {
			if alreadyMappedDisks[strings.ToLower(reqDisk)] {
				disksToUnmap = append(disksToUnmap, reqDisk)
			} else {
				alreadyUnmapped = append(alreadyUnmapped, reqDisk)
			}
		}

		if len(alreadyUnmapped) > 0 {
			fmt.Printf("⚠️  Skipping unmapped disks (already detached from this LPAR):\n")
			for _, d := range alreadyUnmapped {
				fmt.Printf("   - %s\n", d)
			}
		}

		if len(disksToUnmap) == 0 {
			fmt.Printf("\n✅ All requested disks are already detached from LPAR '%s'. No action needed.\n", *lparName)
			operationStatus = "NOT_FOUND"
		} else {
			fmt.Printf("\n⚠️  Attempting to DELETE %d Virtual Disk mapping(s) from LPAR '%s'...\n", len(disksToUnmap), *lparName)
			fmt.Printf("Disks to unmap: %v\n", disksToUnmap)

			operationStatus, err = restClient.DeleteVirtualDiskMaps(sysUUID, viosUUID, lparUUID, disksToUnmap, *verbose)
			if err != nil {
				log.Fatalf("❌ Storage Deletion Failed: %v", err)
			}
			fmt.Printf("\n🗑️  Deletion operation completed! Status: %s\n", operationStatus)
		}

	} else {
		// --- CREATE LOGIC ---
		fmt.Printf("[Validate] Verifying requested Virtual Disks exist on VIOS '%s'...\n", *viosName)
		vgs, err := restClient.GetVolumeGroups(context.Background(), viosUUID, *verbose)
		if err != nil {
			log.Fatalf("❌ Failed to get Volume Groups to validate disks: %v", err)
		}

		availableDisks := make(map[string]bool)
		for _, vg := range vgs {
			for _, vd := range vg.VirtualDisks {
				availableDisks[strings.ToLower(strings.TrimSpace(vd.DiskName))] = true
			}
		}

		var validDisks []string
		var missingDisks []string
		for _, d := range diskNames {
			if availableDisks[strings.ToLower(d)] {
				validDisks = append(validDisks, d)
			} else {
				missingDisks = append(missingDisks, d)
			}
		}

		if len(missingDisks) > 0 {
			fmt.Printf("\n⚠️  Warning: The following Virtual Disks do NOT exist on VIOS '%s' and will be SKIPPED:\n", *viosName)
			for _, d := range missingDisks {
				fmt.Printf("   - %s\n", d)
			}
		}

		if len(validDisks) == 0 {
			log.Fatalf("\n❌ Cannot proceed: None of the requested Virtual Disks exist on the VIOS.")
		}
		
		fmt.Printf("\n✅ %d valid Virtual Disk(s) found on VIOS.\n", len(validDisks))

		var disksToMap []string
		var skippedDisks []string

		for _, reqDisk := range validDisks {
			if alreadyMappedDisks[strings.ToLower(reqDisk)] {
				skippedDisks = append(skippedDisks, reqDisk)
			} else {
				disksToMap = append(disksToMap, reqDisk)
			}
		}

		if len(skippedDisks) > 0 {
			fmt.Printf("⚠️  Skipping already mapped disks:\n")
			for _, d := range skippedDisks {
				fmt.Printf("   - %s\n", d)
			}
		}

		if len(disksToMap) == 0 {
			fmt.Printf("\n✅ All valid requested disks are already mapped to LPAR '%s'. No action needed.\n", *lparName)
			operationStatus = "ALREADY_MAPPED"
		} else {
			fmt.Printf("\n⚠️  Attempting to MAP %d Virtual Disk(s) to LPAR '%s'...\n", len(disksToMap), *lparName)
			fmt.Printf("Disks to map: %v\n", disksToMap)

			operationStatus, err = restClient.CreateVirtualDiskMaps(sysUUID, viosUUID, lparUUID, disksToMap, *verbose)
			if err != nil {
				log.Fatalf("❌ Storage Mapping Failed: %v", err)
			}
			fmt.Printf("\n💾 Mapping operation completed! Status: %s\n", operationStatus)
		}
	}
	// =========================================================================
	// 5. SAVE LPAR PROFILE (Only if topology changes were made)
	// =========================================================================
	if operationStatus == "SUCCESS" || operationStatus == "SUCCESS_WITH_RMC_WARNING" {
		fmt.Printf("\n[Profile] Saving running configuration to LPAR profile '%s'...\n", *lparProfile)
		saveErr := restClient.SaveCurrentLparConfig(context.Background(), lparUUID, *lparProfile, *forceSave, *verbose)
		if saveErr != nil {
			log.Printf("⚠️ Warning: vFC topology modified dynamically, but failed to save LPAR profile: %v\n", saveErr)
		} else {
			fmt.Println("✅ Success: LPAR profile saved. The Client Fibre Channel adapters will persist across reboots.")
		}
	} else {
		fmt.Printf("\n[Profile] No architectural changes were made to the LPAR. Profile save skipped.\n")
	}


	// =========================================================================
	// 4. DUMP "AFTER" XML
	// =========================================================================
	if operationStatus != "NOT_FOUND" && operationStatus != "ALREADY_MAPPED" {
		fmt.Printf("\n[Diff Tool] Fetching 'AFTER' XML state...\n")
		afterXML, err := restClient.GetRawViosXML(viosUUID, "ViosSCSIMapping", *verbose)
		if err != nil {
			log.Fatalf("❌ Failed to fetch after XML: %v", err)
		}

		err = os.WriteFile(afterFile, []byte(afterXML), 0644)
		if err != nil {
			log.Fatalf("❌ Failed to write after file: %v", err)
		}
		fmt.Printf("   -> Saved '%s'\n", afterFile)
	}

	if operationStatus != "NOT_FOUND" && operationStatus != "ALREADY_MAPPED" {
		fmt.Println("\n=========================================================================")
		fmt.Printf(" 🎉 TEST COMPLETE! You can now diff the XML files:\n")
		fmt.Printf("    Linux/Mac: diff %s %s\n", beforeFile, afterFile)
		fmt.Printf("    VS Code:   code --diff %s %s\n", beforeFile, afterFile)
		fmt.Println("=========================================================================")
	}
}