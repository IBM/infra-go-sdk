package main

import (
	"context"
	"encoding/json"
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
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	systemName := flag.String("system-name", "", "Managed system name (required)")
	lparName := flag.String("lpar-name", "", "LPAR name (required)")
	verbose := flag.Bool("verbose", true, "Enable verbose logging")

	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose

	// Validate required parameters
	if *systemName == "" || *lparName == "" {
		log.Fatal("Error: Both -system-name and -lpar-name are required\n\n" +
			"Usage: go run main.go -system-name=<system> -lpar-name=<lpar>\n" +
			"Example: go run main.go -system-name=Server-9080-MHE-SN1234567 -lpar-name=mylpar")
	}

	fmt.Println("=== Get Partition Profile Example ===")
	fmt.Println("This example retrieves all profiles for a logical partition")
	fmt.Println()

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)
	if err := restClient.Login(context.Background(), *username, *password); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// RESOLVE SYSTEM UUID
	// =========================================================================
	fmt.Printf("Resolving managed system: %s\n", *systemName)
	
	systemUUID, system, err := restClient.GetManagedSystemByName(context.Background(), *systemName)
	if err != nil {
		log.Fatalf("❌ Failed to get managed system: %v", err)
	}
	
	fmt.Printf("✅ Found system: %s (UUID: %s)\n", system.SystemName, systemUUID)
	fmt.Println()

	// =========================================================================
	// RESOLVE LPAR UUID
	// =========================================================================
	fmt.Printf("Resolving LPAR: %s\n", *lparName)
	
	lpar, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), systemUUID, *lparName)
	if err != nil {
		log.Fatalf("❌ Failed to get LPAR: %v", err)
	}
	
	fmt.Printf("✅ Found LPAR: %s (UUID: %s)\n", lpar.PartitionName, lparUUID)
	fmt.Printf("   Current State: %s\n", lpar.PartitionState)
	fmt.Printf("   Last Activated Profile: %s\n", lpar.LastActivatedProfile)
	fmt.Println()

	// =========================================================================
	// RETRIEVE PARTITION PROFILES
	// =========================================================================
	fmt.Println("Retrieving partition profiles...")
	fmt.Println()

	profiles, err := restClient.GetLogicalPartitionProfiles(context.Background(), lparUUID)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve partition profiles: %v", err)
	}

	fmt.Printf("✅ Found %d profile(s) for LPAR '%s'\n", len(profiles), *lparName)
	fmt.Println()

	// =========================================================================
	// DISPLAY PROFILES
	// =========================================================================
	for i, profile := range profiles {
		fmt.Printf("Profile %d: %s\n", i+1, profile.ProfileName)
		fmt.Println("─────────────────────────────────────────────────────────────")
		fmt.Printf("  UUID:                    %s\n", profile.UUID)
		fmt.Printf("  Profile Type:            %s\n", profile.ProfileType)
		fmt.Printf("  Boot Mode:               %s\n", profile.BootMode)
		fmt.Printf("  Auto Start:              %s\n", profile.AutoStart)
		fmt.Println()
		
		// Memory Configuration
		fmt.Println("  Memory Configuration:")
		fmt.Printf("    Desired:               %s MB\n", profile.MemoryConfig.DesiredMemory)
		fmt.Printf("    Minimum:               %s MB\n", profile.MemoryConfig.MinimumMemory)
		fmt.Printf("    Maximum:               %s MB\n", profile.MemoryConfig.MaximumMemory)
		fmt.Println()
		
		// Processor Configuration
		fmt.Println("  Processor Configuration:")
		fmt.Printf("    Dedicated Processors:  %s\n", profile.ProcessorConfig.HasDedicatedProcessors)
		fmt.Printf("    Sharing Mode:          %s\n", profile.ProcessorConfig.SharingMode)
		
		if profile.ProcessorConfig.HasDedicatedProcessors == "false" {
			// Shared processor configuration
			fmt.Printf("    Desired Proc Units:    %s\n", profile.ProcessorConfig.SharedConfig.DesiredProcessingUnits)
			fmt.Printf("    Desired vCPUs:         %s\n", profile.ProcessorConfig.SharedConfig.DesiredVirtualProcessors)
			fmt.Printf("    Minimum Proc Units:    %s\n", profile.ProcessorConfig.SharedConfig.MinimumProcessingUnits)
			fmt.Printf("    Maximum Proc Units:    %s\n", profile.ProcessorConfig.SharedConfig.MaximumProcessingUnits)
			fmt.Printf("    Processor Pool:        %s\n", profile.ProcessorConfig.SharedConfig.SharedProcessorPoolName)
			fmt.Printf("    Uncapped Weight:       %s\n", profile.ProcessorConfig.SharedConfig.UncappedWeight)
		} else {
			// Dedicated processor configuration
			fmt.Printf("    Desired Processors:    %s\n", profile.ProcessorConfig.DedicatedConfig.DesiredProcessors)
			fmt.Printf("    Minimum Processors:    %s\n", profile.ProcessorConfig.DedicatedConfig.MinimumProcessors)
			fmt.Printf("    Maximum Processors:    %s\n", profile.ProcessorConfig.DedicatedConfig.MaximumProcessors)
		}
		fmt.Println()
		
		// I/O Configuration
		fmt.Println("  I/O Configuration:")
		fmt.Printf("    Max Virtual I/O Slots: %s\n", profile.MaximumVirtualIOSlots)
		fmt.Println()
		
		// Display full JSON for detailed inspection
		fmt.Println("  Full Profile Data (JSON):")
		jsonData, err := json.MarshalIndent(profile, "    ", "  ")
		if err != nil {
			fmt.Printf("    Error marshaling to JSON: %v\n", err)
		} else {
			fmt.Printf("    %s\n", string(jsonData))
		}
		fmt.Println()
	}

	fmt.Printf("Total profiles retrieved: %d\n", len(profiles))
}

// Made with Bob
