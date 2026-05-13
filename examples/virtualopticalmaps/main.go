package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
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
	lparName := flag.String("lpar-name", "TEST-CLOUD-INIT-ISO", "Target LPAR Name")
	lparProfile := flag.String("lpar-profile", "default_profile", "Name of the LPAR profile to overwrite")
	forceSave := flag.Bool("force-save", true, "Force overwrite of the target profile")
	mediaNamesStr := flag.String("media-names", "ocp_1774847136", "Comma-separated list of ISO files to map/unmap")

	deleteMode := flag.Bool("delete", false, "Set to true to DELETE the mappings instead of creating them")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	flag.Parse()

	if *password == "" || *viosName == "" || *lparName == "" || *mediaNamesStr == "" {
		log.Fatal("Error: hmc-pass, vios-name, lpar-name, and media-names are required.")
	}

	mediaNames := strings.Split(*mediaNamesStr, ",")
	for i := range mediaNames {
		mediaNames[i] = strings.TrimSpace(mediaNames[i])
	}

	// =========================================================================
	// DIRECTORY & FILENAME PREPARATION
	// =========================================================================
	outDir := "outs"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("❌ Failed to create output directory: %v", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	safeMediaTag := strings.ReplaceAll(strings.TrimSpace(*mediaNamesStr), ",", "-")
	safeMediaTag = strings.ReplaceAll(safeMediaTag, " ", "")

	beforeFile := fmt.Sprintf("%s/vios_before_%s_%s.xml", outDir, safeMediaTag, timestamp)
	afterFile := fmt.Sprintf("%s/vios_after_%s_%s.xml", outDir, safeMediaTag, timestamp)

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	sysUUID, _, err := restClient.GetManagedSystemByName(context.Background(), *sysName, *verbose)
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
	alreadyMappedMedia := make(map[string]bool)

	// Build a fast-lookup map of what is currently attached
	for _, mapping := range mappings {
		if strings.HasSuffix(strings.ToLower(mapping.AssociatedLogicalPartition.Href), targetLparLower) {
			mappedName := strings.TrimSpace(mapping.Storage.VirtualOpticalMedia.MediaName)
			if mappedName != "" {
				alreadyMappedMedia[strings.ToLower(mappedName)] = true
			}
		}
	}

	// =========================================================================
	// 3. EXECUTE THE OPERATION (CREATE OR DELETE)
	// =========================================================================
	var operationStatus string

	if *deleteMode {
		// --- DELETE LOGIC ---
		var mediaToUnmap []string
		var alreadyUnmapped []string

		for _, reqMedia := range mediaNames {
			if alreadyMappedMedia[strings.ToLower(reqMedia)] {
				mediaToUnmap = append(mediaToUnmap, reqMedia)
			} else {
				alreadyUnmapped = append(alreadyUnmapped, reqMedia)
			}
		}

		if len(alreadyUnmapped) > 0 {
			fmt.Printf("⚠️  Skipping unmapped media (already detached):\n")
			for _, m := range alreadyUnmapped {
				fmt.Printf("   - %s\n", m)
			}
		}

		if len(mediaToUnmap) == 0 {
			fmt.Printf("\n✅ All requested media is already detached from LPAR '%s'. No action needed.\n", *lparName)
			operationStatus = "NOT_FOUND"
		} else {
			fmt.Printf("\n⚠️  Attempting to DELETE %d Virtual Optical Media mapping(s) from LPAR '%s'...\n", len(mediaToUnmap), *lparName)
			fmt.Printf("Media to unmap: %v\n", mediaToUnmap)

			operationStatus, err = restClient.DeleteVirtualOpticalMaps(context.Background(), sysUUID, viosUUID, lparUUID, mediaToUnmap, *verbose)
			if err != nil {
				log.Fatalf("❌ Storage Deletion Failed: %v", err)
			}
			fmt.Printf("\n🗑️  Deletion operation completed! Status: %s\n", operationStatus)
		}

	} else {
		// --- CREATE LOGIC ---
		
		// 3a. Validate media actually exists in the VIOS repository
		fmt.Printf("[Validate] Verifying requested media exists in VIOS repository...\n")
		viosDetails, err := restClient.GetVirtualIOServer(context.Background(), viosUUID, *verbose)
		if err != nil {
			log.Fatalf("❌ Failed to get VIOS details: %v", err)
		}

		availableMedia := make(map[string]bool)
		for _, repo := range viosDetails.MediaRepositories {
			for _, media := range repo.VirtualOpticalMedia {
				availableMedia[strings.ToLower(media.MediaName)] = true
			}
		}

		var validMedia []string
		for _, m := range mediaNames {
			if availableMedia[strings.ToLower(m)] {
				validMedia = append(validMedia, m)
			} else {
				log.Fatalf("\n❌ Cannot proceed: ISO '%s' not found in VIOS repository.", m)
			}
		}
		
		// 3b. Filter out what is already mapped
		var mediaToMap []string
		var skippedMedia []string

		for _, reqMedia := range validMedia {
			if alreadyMappedMedia[strings.ToLower(reqMedia)] {
				skippedMedia = append(skippedMedia, reqMedia)
			} else {
				mediaToMap = append(mediaToMap, reqMedia)
			}
		}

		if len(skippedMedia) > 0 {
			fmt.Printf("⚠️  Skipping already mapped media:\n")
			for _, m := range skippedMedia {
				fmt.Printf("   - %s\n", m)
			}
		}

		if len(mediaToMap) == 0 {
			fmt.Printf("\n✅ All requested media is already mapped to LPAR '%s'. No action needed.\n", *lparName)
			operationStatus = "ALREADY_MAPPED"
		} else {
			fmt.Printf("\n⚠️  Attempting to MAP %d Virtual Optical Media to LPAR '%s'...\n", len(mediaToMap), *lparName)
			fmt.Printf("Media to map: %v\n", mediaToMap)

			// Assuming CreateVirtualOpticalMaps is your auto-pilot creation function
			operationStatus, err = restClient.CreateVirtualOpticalMaps(context.Background(), sysUUID, viosUUID, lparUUID, mediaToMap, *verbose)
			if err != nil {
				log.Fatalf("❌ Storage Mapping Failed: %v", err)
			}
			fmt.Printf("\n💿 Mapping operation completed! Status: %s\n", operationStatus)
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

	fmt.Println("\n=========================================================================")
	fmt.Printf(" 🎉 TEST COMPLETE! You can now diff the XML files:\n")
	fmt.Printf("    Linux/Mac: diff %s %s\n", beforeFile, afterFile)
	fmt.Printf("    VS Code:   code --diff %s %s\n", beforeFile, afterFile)
	fmt.Println("=========================================================================")
}