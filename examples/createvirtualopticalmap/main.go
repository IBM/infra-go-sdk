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
	lparName := flag.String("lpar-name", "TEST-CLOUD-INIT-ISO", "Target LPAR Name")
	mediaNamesStr := flag.String("media-names", "ocp_1774847136", "Comma-separated list of ISO files to map (e.g., 'rhel9.iso,aix73.iso')")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *viosName == "" || *lparName == "" || *mediaNamesStr == "" {
		log.Fatal("Error: hmc-pass, vios-name, lpar-name, and media-names are required.")
	}

	// Parse comma-separated media names
	mediaNames := strings.Split(*mediaNamesStr, ",")
	for i := range mediaNames {
		mediaNames[i] = strings.TrimSpace(mediaNames[i])
	}

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	sysUUID, _, err := restClient.GetManagedSystemByName(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	viosUUID, err := hmc.GetViosID(restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found.", *viosName)
	}

	_,lparUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}

	// =========================================================================
	// VALIDATE MEDIA EXISTS IN VIOS REPOSITORY
	// =========================================================================
	fmt.Printf("\n[Validate] Checking if requested media exists in VIOS '%s' repository...\n", *viosName)
	
	viosDetails, err := restClient.GetVirtualIOServer(viosUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get VIOS details: %v", err)
	}
	
	// Build a map of available media from all media repositories
	availableMedia := make(map[string]bool)
	for _, repo := range viosDetails.MediaRepositories {
		for _, media := range repo.VirtualOpticalMedia {
			availableMedia[media.MediaName] = true
		}
	}
	
	if len(availableMedia) == 0 {
		log.Fatalf("❌ No optical media found in VIOS '%s' repository. Please add media first using AddVirtualOpticalMedia.", *viosName)
	}
	
	// Validate each requested media exists
	var missingMedia []string
	var validMedia []string
	
	for _, mediaName := range mediaNames {
		if availableMedia[mediaName] {
			validMedia = append(validMedia, mediaName)
		} else {
			missingMedia = append(missingMedia, mediaName)
		}
	}
	
	// Report validation results
	if len(missingMedia) > 0 {
		fmt.Printf("\n❌ Error: The following media not found in VIOS repository:\n")
		for _, media := range missingMedia {
			fmt.Printf("   - %s\n", media)
		}
		fmt.Printf("\nAvailable media in repository:\n")
		for media := range availableMedia {
			fmt.Printf("   - %s\n", media)
		}
		log.Fatalf("\n❌ Cannot proceed: %d media file(s) not found in repository.", len(missingMedia))
	}
	
	fmt.Printf("✅ All requested media validated successfully\n")

	// =========================================================================
	// CHECK IF MEDIA IS ALREADY MAPPED TO THIS LPAR
	// =========================================================================
	fmt.Printf("\n[Check] Verifying if any media is already mapped to LPAR '%s'...\n", *lparName)
	
	// Get current SCSI mappings for this VIOS
	mappings, err := restClient.GetViosSCSIMappings(viosUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get VIOS mappings: %v", err)
	}
	
	// Build a map of already mapped optical media for this LPAR
	alreadyMapped := make(map[string]bool)
	targetLparLower := strings.ToLower(lparUUID)
	
	for _, mapping := range mappings {
		// Check if this mapping belongs to our target LPAR
		if strings.HasSuffix(strings.ToLower(mapping.AssociatedLogicalPartition.Href), targetLparLower) {
			// Check if it's optical media
			if mapping.Storage.VirtualOpticalMedia.MediaName != "" {
				alreadyMapped[mapping.Storage.VirtualOpticalMedia.MediaName] = true
			}
		}
	}
	
	// Filter out already mapped media
	var mediaToMap []string
	var skippedMedia []string
	
	for _, mediaName := range validMedia {
		if alreadyMapped[mediaName] {
			skippedMedia = append(skippedMedia, mediaName)
		} else {
			mediaToMap = append(mediaToMap, mediaName)
		}
	}
	
	// Report findings
	if len(skippedMedia) > 0 {
		fmt.Printf("⚠️  Skipping already mapped media:\n")
		for _, media := range skippedMedia {
			fmt.Printf("   - %s (already mapped to this LPAR)\n", media)
		}
	}
	
	if len(mediaToMap) == 0 {
		fmt.Printf("\n✅ All requested media is already mapped to LPAR '%s'. No action needed.\n", *lparName)
		return
	}
	
	fmt.Printf("✅ %d media file(s) ready to map\n", len(mediaToMap))

	// =========================================================================
	// EXECUTE OPTICAL MAPPING (BATCH OPERATION)
	// =========================================================================
	fmt.Printf("\n⚠️  Attempting to map %d Virtual Optical Media from VIOS '%s' to LPAR '%s'...\n", len(mediaToMap), *viosName, *lparName)
	fmt.Printf("Media to map: %v\n", mediaToMap)

	mappingUUID, err := restClient.CreateVirtualOpticalMaps(sysUUID, viosUUID, lparUUID, mediaToMap, *verbose)
	if err != nil {
		log.Fatalf("❌ Storage Mapping Failed: %v", err)
	}

	fmt.Printf("\n💿 Successfully created %d Virtual Optical Device(s) and loaded media! Status: %s\n", len(mediaToMap), mappingUUID)
	
	if len(skippedMedia) > 0 {
		fmt.Printf("\nNote: %d media file(s) were already mapped and skipped.\n", len(skippedMedia))
	}
}
