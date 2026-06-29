package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
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

	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose

	if *password == "" || *sysName == "" || *lparName == "" || *profileName == "" {
		log.Fatal("Error: hmc-pass, system-name, lpar-name, and profile-name are required.")
	}

	log.Println("=========================================================================")
	log.Printf(" 💾 Saving Active Configuration for LPAR '%s'", *lparName)
	log.Println("=========================================================================")

	// =========================================================================
	// 1. AUTHENTICATION
	// =========================================================================
	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)
	if err := restClient.Login(context.Background(), *username, *password); err != nil {
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// 2. RESOLVE SYSTEM AND LPAR UUIDS
	// =========================================================================
	fmt.Printf("🔍 Resolving System '%s' and LPAR '%s'...\n", *sysName, *lparName)

	// Resolve System UUID
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	// Resolve LPAR UUID
	_,lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found on System '%s'.", *lparName, *sysName)
	}

	// =========================================================================
	// 3. EXECUTE SAVE JOB
	// =========================================================================
	fmt.Printf("🚀 Initiating SaveCurrentConfig Job (Target Profile: '%s', Force: %t)...\n", *profileName, *force)

	err = restClient.SaveCurrentLparConfig(context.Background(), lparUUID, *profileName, *force)
	if err != nil {
		log.Fatalf("❌ Failed to save configuration: %v", err)
	}

	fmt.Printf("\n🎉 SUCCESS: The active configuration for LPAR '%s' has been permanently saved to '%s'!\n", *lparName, *profileName)
	fmt.Println("=========================================================================")
}
