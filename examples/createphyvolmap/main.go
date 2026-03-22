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
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS")
	
	// Additional flags for storage mapping
	lparName := flag.String("lpar-name", "Go_LPAR_99", "Target LPAR Name")
	diskNamesStr := flag.String("disk-names", "hdisk3,hdisk4", "Comma-separated list of physical volumes on the VIOS (e.g., 'hdisk3,hdisk4,hdisk5')")
	
	// NEW: Flags for profile saving
	viosProfile := flag.String("vios-profile", "default_profile", "Name of the VIOS profile to overwrite")
	forceSave := flag.Bool("force-save", true, "Force overwrite of the target profile")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *viosName == "" || *lparName == "" || *diskNamesStr == "" {
		log.Fatal("Error: hmc-pass, vios-name, lpar-name, and disk-names are required.")
	}

	// Parse comma-separated disk names
	diskNames := strings.Split(*diskNamesStr, ",")
	for i := range diskNames {
		diskNames[i] = strings.TrimSpace(diskNames[i])
	}

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	systems, _, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
	if err != nil || systems.UUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	viosUUID, err := hmc.GetViosID(restClient, systems.UUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found.", *viosName)
	}

	// Note: Adjust this line to match your exact LPAR resolver function name 
	// (e.g., GetLogicalPartitionByName or hmc.GetLparID)
	_, lparUUID, err := restClient.GetLogicalPartitionByName(systems.UUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}

	// =========================================================================
	// VERIFY WHICH DISKS ARE ALREADY MAPPED
	// =========================================================================
	fmt.Printf("\n[Verify] Checking which disks are currently mapped...\n")
	
	// Get all SCSI mappings for this VIOS
	mappings, err := restClient.GetViosSCSIMappings(viosUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get VIOS mappings: %v", err)
	}
	
	// Build a map of currently mapped disks for this LPAR
	mappedDisks := make(map[string]bool)
	targetLparLower := strings.ToLower(lparUUID)
	
	for _, mapping := range mappings {
		// Check if this mapping belongs to our target LPAR
		if strings.HasSuffix(strings.ToLower(mapping.AssociatedLparURI), targetLparLower) {
			diskName := mapping.ServerAdapter.BackingDeviceName
			if diskName == "" && mapping.Storage.VolumeName != "" {
				diskName = mapping.Storage.VolumeName
			}
			if diskName != "" {
				mappedDisks[diskName] = true
			}
		}
	}
	
	// Filter disks to only those that are NOT already mapped
	var disksToMap []string
	var alreadyMappedDisks []string
	
	for _, disk := range diskNames {
		if mappedDisks[disk] {
			alreadyMappedDisks = append(alreadyMappedDisks, disk)
		} else {
			disksToMap = append(disksToMap, disk)
		}
	}
	
	// Report findings
	if len(alreadyMappedDisks) > 0 {
		fmt.Printf("⚠️  Skipping disks (already mapped): %v\n", alreadyMappedDisks)
	}
	
	if len(disksToMap) == 0 {
		fmt.Printf("\n⚠️ Notice: All specified disks are already mapped. No changes needed.\n")
		fmt.Println("=========================================================================")
		return
	}
	
	fmt.Printf("✅ Found %d disk(s) to map: %v\n", len(disksToMap), disksToMap)

	// =========================================================================
	// EXECUTE STORAGE MAPPING (BATCH OPERATION)
	// =========================================================================
	fmt.Printf("\n⚠️  Mapping %d Physical Volume(s) from VIOS '%s' to LPAR '%s'...\n", len(disksToMap), *viosName, *lparName)

	mappingUUID, err := restClient.CreatePhysicalVolumeMap(systems.UUID, viosUUID, lparUUID, disksToMap, *verbose)
	if err != nil {
		log.Fatalf("❌ Storage Mapping Failed: %v", err)
	}

	fmt.Printf("\n💾 Successfully mapped %d Physical Volume(s). Status: %s\n", len(disksToMap), mappingUUID)
	
	// =========================================================================
	// SAVE PROFILE (PERSIST CHANGES)
	// =========================================================================
	fmt.Printf("\n[Profile] Saving running configuration to LPAR profile '%s'...\n", *viosProfile)
	
	// Save the LPAR configuration to persist the mapping
	saveErr := restClient.SaveCurrentLparConfig(lparUUID, *viosProfile, *forceSave, *verbose)
	if saveErr != nil {
		log.Printf("⚠️ Warning: Disk mapped dynamically, but failed to save LPAR profile: %v\n", saveErr)
	} else {
		fmt.Println("✅ Success: LPAR profile saved. The new mapping will persist across reboots.")
	}
}