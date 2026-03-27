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
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Name of the VIOS hosting the optical media")
	viosProfile := flag.String("vios-profile", "default_profile", "Name of the VIOS profile to overwrite")
	
	// Virtual Optical Media (ISO files)
	mediaNamesStr := flag.String("media-names", "rhel9.iso,aix73.iso", "Comma-separated list of Virtual Optical Media (ISO files) to unmap")
	
	verbose := flag.Bool("verbose", true, "Enable verbose output")
	forceSave := flag.Bool("force-save", true, "Force overwrite of the target profile")
	flag.Parse()

	if *password == "" || *mediaNamesStr == "" {
		log.Fatal("❌ Error: hmc-pass and media-names are required.")
	}

	// Parse comma-separated list into a slice
	mediaToUnmap := strings.Split(*mediaNamesStr, ",")
	for i := range mediaToUnmap {
		mediaToUnmap[i] = strings.TrimSpace(mediaToUnmap[i])
	}

	fmt.Println("=========================================================================")
	fmt.Printf(" 🗑️  Starting Bulk Virtual Optical Media Unmapping for %d Media\n", len(mediaToUnmap))
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
	// 2. VERIFY VIRTUAL OPTICAL MAPPINGS EXIST
	// =========================================================================
	fmt.Printf("\n[Verify] Checking which virtual optical media are currently mapped...\n")
	
	// Get all SCSI mappings for this VIOS
	mappings, err := restClient.GetViosSCSIMappings(viosUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get VIOS mappings: %v", err)
	}
	
	// Build a map of currently mapped optical media for this LPAR
	mappedMedia := make(map[string]bool)
	targetLparLower := strings.ToLower(lparUUID)
	
	for _, mapping := range mappings {
		// Check if this mapping belongs to our target LPAR
		if strings.HasSuffix(strings.ToLower(mapping.AssociatedLogicalPartition.Href), targetLparLower) {
			// For virtual optical media, check the MediaName field
			mediaName := mapping.Storage.VirtualOpticalMedia.MediaName
			if mediaName != "" {
				mappedMedia[mediaName] = true
			}
		}
	}
	
	// Filter media to only those that are actually mapped
	var mediaToDelete []string
	var notMappedMedia []string
	
	for _, media := range mediaToUnmap {
		if mappedMedia[media] {
			mediaToDelete = append(mediaToDelete, media)
		} else {
			notMappedMedia = append(notMappedMedia, media)
		}
	}
	
	// Report findings
	if len(notMappedMedia) > 0 {
		fmt.Printf("⚠️  Skipping optical media (not mapped): %v\n", notMappedMedia)
	}
	
	if len(mediaToDelete) == 0 {
		fmt.Printf("\n⚠️ Notice: None of the specified optical media are currently mapped. No changes needed.\n")
		fmt.Println("=========================================================================")
		return
	}
	
	fmt.Printf("✅ Found %d optical media to unmap: %v\n", len(mediaToDelete), mediaToDelete)
	
	// =========================================================================
	// 3. EXECUTE BULK UNMAP OPERATION
	// =========================================================================
	fmt.Printf("\n[Unmap] Detaching optical media %v from LPAR '%s' via VIOS '%s'...\n", mediaToDelete, *lparName, *viosName)
	
	status, err := restClient.DeleteVirtualOpticalMaps(sysUUID, viosUUID, lparUUID, mediaToDelete, *verbose)
	if err != nil {
		log.Fatalf("❌ Bulk Virtual Optical Media Unmap Operation Failed: %v", err)
	}

	// =========================================================================
	// 4. SAVE PROFILE (PERSIST CHANGES)
	// =========================================================================
	if status == "SUCCESS" {
		fmt.Printf("\n[Profile] Saving running configuration to VIOS profile '%s'...\n", *viosProfile)
		
		saveErr := restClient.SaveCurrentLparConfig(viosUUID, *viosProfile, *forceSave, *verbose)
		if saveErr != nil {
			log.Printf("⚠️ Warning: Optical media unmapped dynamically, but failed to save VIOS profile: %v\n", saveErr)
		} else {
			fmt.Println("✅ Success: VIOS profile saved. Optical media mapping removals will persist across reboots.")
		}
	} else if status == "NOT_FOUND" {
		fmt.Printf("\n⚠️ Notice: None of the specified optical media were mapped. No changes made.\n")
	}
	
	fmt.Println("=========================================================================")
}

// Made with Bob
