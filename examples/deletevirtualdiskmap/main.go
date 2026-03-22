package main

import (
	"flag"
	"fmt"
	"log"
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
	
	// Dynamic Target Identifiers
	lparName := flag.String("lpar-name", "Go_LPAR_99", "Name of the Client LPAR")
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Name of the VIOS hosting the virtual disks")
	viosProfile := flag.String("vios-profile", "default_profile", "Name of the VIOS profile to overwrite")
	
	// Virtual Disks (Logical Volumes)
	diskNamesStr := flag.String("disk-names", "lv01,lv02", "Comma-separated list of Virtual Disks (logical volumes) to unmap")
	
	verbose := flag.Bool("verbose", true, "Enable verbose output")
	forceSave := flag.Bool("force-save", true, "Force overwrite of the target profile")
	flag.Parse()

	if *password == "" || *diskNamesStr == "" {
		log.Fatal("❌ Error: hmc-pass and disk-names are required.")
	}

	// Parse comma-separated list into a slice
	disksToUnmap := strings.Split(*diskNamesStr, ",")
	for i := range disksToUnmap {
		disksToUnmap[i] = strings.TrimSpace(disksToUnmap[i])
	}

	fmt.Println("=========================================================================")
	fmt.Printf(" 🗑️  Starting Bulk Virtual Disk Unmapping for %d Disks\n", len(disksToUnmap))
	fmt.Println("=========================================================================")

	// =========================================================================
	// 1. AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("❌ HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	systems, _, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
	if err != nil || systems.UUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}
	sysUUID := systems.UUID

	_, lparUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}

	viosUUID, err := hmc.GetViosID(restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found.", *viosName)
	}

	// =========================================================================
	// 2. VERIFY VIRTUAL DISK MAPPINGS EXIST
	// =========================================================================
	fmt.Printf("\n[Verify] Checking which virtual disks are currently mapped...\n")
	
	// Get all SCSI mappings for this VIOS
	mappings, err := restClient.GetViosSCSIMappingDetails(viosUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get VIOS mappings: %v", err)
	}
	
	// Build a map of currently mapped virtual disks for this LPAR
	mappedDisks := make(map[string]bool)
	targetLparLower := strings.ToLower(lparUUID)
	
	for _, mapping := range mappings {
		// Check if this mapping belongs to our target LPAR
		if strings.HasSuffix(strings.ToLower(mapping.AssociatedLparURI), targetLparLower) {
			// For virtual disks, use the BackingDeviceName from ServerAdapter
			// Virtual disks typically start with "lv" (logical volumes)
			diskName := mapping.ServerAdapter.BackingDeviceName
			if diskName != "" && strings.HasPrefix(diskName, "lv") {
				mappedDisks[diskName] = true
			}
		}
	}
	
	// Filter disks to only those that are actually mapped
	var disksToDelete []string
	var notMappedDisks []string
	
	for _, disk := range disksToUnmap {
		if mappedDisks[disk] {
			disksToDelete = append(disksToDelete, disk)
		} else {
			notMappedDisks = append(notMappedDisks, disk)
		}
	}
	
	// Report findings
	if len(notMappedDisks) > 0 {
		fmt.Printf("⚠️  Skipping virtual disks (not mapped): %v\n", notMappedDisks)
	}
	
	if len(disksToDelete) == 0 {
		fmt.Printf("\n⚠️ Notice: None of the specified virtual disks are currently mapped. No changes needed.\n")
		fmt.Println("=========================================================================")
		return
	}
	
	fmt.Printf("✅ Found %d virtual disk(s) to unmap: %v\n", len(disksToDelete), disksToDelete)
	
	// =========================================================================
	// 3. EXECUTE BULK UNMAP OPERATION
	// =========================================================================
	fmt.Printf("\n[Unmap] Detaching virtual disks %v from LPAR '%s' via VIOS '%s'...\n", disksToDelete, *lparName, *viosName)
	
	status, err := restClient.DeleteVirtualDiskMaps(sysUUID, viosUUID, lparUUID, disksToDelete, *verbose)
	if err != nil {
		log.Fatalf("❌ Bulk Virtual Disk Unmap Operation Failed: %v", err)
	}

	// =========================================================================
	// 4. SAVE PROFILE (PERSIST CHANGES)
	// =========================================================================
	if status == "SUCCESS" {
		fmt.Printf("\n[Profile] Saving running configuration to VIOS profile '%s'...\n", *viosProfile)
		
		saveErr := restClient.SaveCurrentLparConfig(viosUUID, *viosProfile, *forceSave, *verbose)
		if saveErr != nil {
			log.Printf("⚠️ Warning: Virtual disks unmapped dynamically, but failed to save VIOS profile: %v\n", saveErr)
		} else {
			fmt.Println("✅ Success: VIOS profile saved. Virtual disk mapping removals will persist across reboots.")
		}
	} else if status == "NOT_FOUND" {
		fmt.Printf("\n⚠️ Notice: None of the specified virtual disks were mapped. No changes made.\n")
	}
	
	fmt.Println("=========================================================================")
}

// Made with Bob
