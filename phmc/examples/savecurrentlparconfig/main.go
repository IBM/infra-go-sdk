package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC Username")
	password := flag.String("hmc-pass", "", "HMC Password")
	
	sysName := flag.String("system-name", "", "Managed System Name")
	lparName := flag.String("lpar-name", "", "Name of the LPAR to save")
	
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
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// 2. RESOLVE SYSTEM AND LPAR UUIDS
	// =========================================================================
	fmt.Printf("🔍 Resolving System '%s' and LPAR '%s'...\n", *sysName, *lparName)
	
	// Resolve System UUID
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	// Resolve LPAR UUID
	_,lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found on System '%s'.", *lparName, *sysName)
	}

	// =========================================================================
	// 3. EXECUTE SAVE JOB
	// =========================================================================
	fmt.Printf("🚀 Initiating SaveCurrentConfig Job (Target Profile: '%s', Force: %t)...\n", *profileName, *force)
	
	err = restClient.SaveCurrentLparConfig(context.Background(), lparUUID, *profileName, *force, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to save configuration: %v", err)
	}

	fmt.Printf("\n🎉 SUCCESS: The active configuration for LPAR '%s' has been permanently saved to '%s'!\n", *lparName, *profileName)
	fmt.Println("=========================================================================")
}
