package main

import (
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC Username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC Password")
	
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	lparName := flag.String("lpar-name", "Go_LPAR_03", "Name of the LPAR to save")
	
	profileName := flag.String("profile-name", "default_profile", "Name of the target profile")
	force := flag.Bool("force", true, "Overwrite the profile if it already exists")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	flag.Parse()

	if *password == "" || *sysName == "" || *lparName == "" || *profileName == "" {
		log.Fatal("Error: hmc-pass, system-name, lpar-name, and profile-name are required.")
	}

	log.Println("=========================================================================")
	log.Printf(" 💾 Saving Active Configuration for LPAR '%s'", *lparName)
	log.Println("=========================================================================")

	// =========================================================================
	// 1. AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff()

	// =========================================================================
	// 2. RESOLVE SYSTEM AND LPAR UUIDS
	// =========================================================================
	fmt.Printf("🔍 Resolving System '%s' and LPAR '%s'...\n", *sysName, *lparName)
	
	// Resolve System UUID
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	// Resolve LPAR UUID
	_,lparUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found on System '%s'.", *lparName, *sysName)
	}

	// =========================================================================
	// 3. EXECUTE SAVE JOB
	// =========================================================================
	fmt.Printf("🚀 Initiating SaveCurrentConfig Job (Target Profile: '%s', Force: %t)...\n", *profileName, *force)
	
	err = restClient.SaveCurrentLparConfig(lparUUID, *profileName, *force, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to save configuration: %v", err)
	}

	fmt.Printf("\n🎉 SUCCESS: The active configuration for LPAR '%s' has been permanently saved to '%s'!\n", *lparName, *profileName)
	fmt.Println("=========================================================================")
}
