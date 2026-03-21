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
	lparName := flag.String("lpar-name", "Go_LPAR_03", "Target LPAR Name")
	locCodesRaw := flag.String("loc-codes", "U78D2.001.WZS0B89-P1-C2,U78D2.001.WZS0B89-P1-C4", "Comma-separated Location Codes")
	
	// NEW: Profile saving flags
	profileName := flag.String("profile-name", "default_profile", "Profile name to save configuration into") 
	forceSave := flag.Bool("force-save", true, "Overwrite existing profile if it exists") 
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *sysName == "" || *lparName == "" || *locCodesRaw == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, lpar-name, and loc-codes are required.")
	}

	locCodes := strings.Split(*locCodesRaw, ",")

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err) 
	}
	defer restClient.Logoff()

	sysUUID, sysDetailed, err := restClient.GetManagedSystemByName(*sysName, *verbose) 
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	_,lparUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose) 
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}

	// =========================================================================
	// BATCH LOOKUP ADAPTER IDs
	// =========================================================================
	var targetAdapterIDs []string
	fmt.Printf("\n🔍 Resolving %d Location Codes and checking availability...\n", len(locCodes))
	
	for _, code := range locCodes {
		cleanCode := strings.TrimSpace(code)
		found := false

		for _, bus := range sysDetailed.IOConfig.IOBuses { 
			for _, slot := range bus.IOSlots { 
				adapter := slot.RelatedIOAdapter 
				if adapter.DeviceName == cleanCode || slot.PhysicalLocationCode == cleanCode {
					found = true
					if !adapter.LogicalPartitionAssignmentCapable { 
						fmt.Printf("   ❌ Skipping %s: Reserved by Hypervisor.\n", cleanCode)
						break
					}
					// Pre-flight ownership check is now handled inside MapPhysicalIOAdapters
					targetAdapterIDs = append(targetAdapterIDs, adapter.AdapterID) 
					fmt.Printf("   ✅ Available: %s -> Adapter ID: %s\n", cleanCode, adapter.AdapterID)
					break
				}
			}
			if found { break }
		}
	}

	if len(targetAdapterIDs) == 0 {
		log.Fatal("❌ No valid, assignable adapters found. Aborting.")
	}

	// =========================================================================
	// EXECUTE BATCH MAPPING
	// =========================================================================
	fmt.Printf("\n🚀 Assigning %d Physical Adapter(s) to LPAR '%s'...\n", len(targetAdapterIDs), *lparName)

	status, err := restClient.MapPhysicalIOAdapters(sysUUID, lparUUID, targetAdapterIDs, sysDetailed, *verbose)
	if err != nil {
		log.Fatalf("❌ Mapping failed: %v", err)
	}

	if status == "ALREADY_MAPPED" {
		fmt.Printf("⚠️  All provided adapters were already assigned to '%s'.\n", *lparName)
	} else {
		fmt.Printf("✅ Success: Adapters mapped successfully.\n")
		
		// =====================================================================
		// SAVE TO PROFILE
		// =====================================================================
		fmt.Printf("💾 Saving active configuration to profile '%s'...\n", *profileName)
		err = restClient.SaveCurrentLparConfig(lparUUID, *profileName, *forceSave, *verbose)
		if err != nil {
			log.Fatalf("❌ Failed to save configuration to profile: %v", err)
		}
		fmt.Printf("🎉 ALL COMPLETE: Adapters mapped and profile '%s' updated for '%s'!\n", *profileName, *lparName)
	}
}