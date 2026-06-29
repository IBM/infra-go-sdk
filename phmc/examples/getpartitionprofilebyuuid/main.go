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
	systemName := flag.String("system-name", "", "Managed system name")
	lparName := flag.String("lpar-name", "", "Name of the LPAR")
	verbose := flag.Bool("verbose", true, "Enable verbose logging")

	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose

	fmt.Println("=== Get Single Partition Profile by UUID Example ===")
	fmt.Println("This example retrieves a specific profile using GetLogicalPartitionProfile()")
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
	
	_, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), systemUUID, *lparName)
	if err != nil {
		log.Fatalf("❌ Failed to get LPAR: %v", err)
	}
	
	fmt.Printf("✅ Found LPAR: %s (UUID: %s)\n", *lparName, lparUUID)
	fmt.Println()

	// =========================================================================
	// GET PROFILE LIST TO FIND FIRST PROFILE UUID
	// =========================================================================
	fmt.Println("Retrieving profile list...")
	
	quickProfiles, err := restClient.GetPartitionProfiles(lparUUID)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve partition profiles: %v", err)
	}

	if len(quickProfiles) == 0 {
		log.Fatalf("❌ No profiles found for LPAR '%s'", *lparName)
	}

	// Use the first profile
	profileUUID := quickProfiles[0].UUID
	profileName := quickProfiles[0].ProfileName

	fmt.Printf("✅ Found %d profile(s), using: %s (UUID: %s)\n", len(quickProfiles), profileName, profileUUID)
	fmt.Println()

	// =========================================================================
	// RETRIEVE SPECIFIC PARTITION PROFILE USING NEW API
	// =========================================================================
	fmt.Printf("Retrieving detailed profile information using GetLogicalPartitionProfile()...\n")
	fmt.Println()

	profile, err := restClient.GetLogicalPartitionProfile(lparUUID, profileUUID)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve partition profile: %v", err)
	}

	fmt.Printf("✅ Successfully retrieved profile: %s\n", profile.ProfileName)
	fmt.Println()

	// =========================================================================
	// DISPLAY PROFILE DETAILS
	// =========================================================================
	fmt.Println("Profile Details:")
	fmt.Println("═════════════════════════════════════════════════════════════")
	fmt.Printf("  Name:                    %s\n", profile.ProfileName)
	fmt.Printf("  UUID:                    %s\n", profile.UUID)
	fmt.Printf("  Profile Type:            %s\n", profile.ProfileType)
	fmt.Printf("  Setting ID:              %s\n", profile.SettingID)
	fmt.Printf("  Created:                 %s\n", profile.AtomCreated)
	fmt.Printf("  Boot Mode:               %s\n", profile.BootMode)
	fmt.Printf("  Auto Start:              %s\n", profile.AutoStart)
	fmt.Println()
	
	// Memory Configuration
	fmt.Println("Memory Configuration:")
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Printf("  Desired:                 %s MB\n", profile.MemoryConfig.DesiredMemory)
	fmt.Printf("  Minimum:                 %s MB\n", profile.MemoryConfig.MinimumMemory)
	fmt.Printf("  Maximum:                 %s MB\n", profile.MemoryConfig.MaximumMemory)
	if profile.MemoryConfig.ActiveMemoryExpansionEnabled != "" {
		fmt.Printf("  AME Enabled:             %s\n", profile.MemoryConfig.ActiveMemoryExpansionEnabled)
	}
	if profile.MemoryConfig.ActiveMemorySharingEnabled != "" {
		fmt.Printf("  AMS Enabled:             %s\n", profile.MemoryConfig.ActiveMemorySharingEnabled)
	}
	fmt.Println()
	
	// Processor Configuration
	fmt.Println("Processor Configuration:")
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Printf("  Dedicated Processors:    %s\n", profile.ProcessorConfig.HasDedicatedProcessors)
	fmt.Printf("  Sharing Mode:            %s\n", profile.ProcessorConfig.SharingMode)
	
	if profile.ProcessorConfig.HasDedicatedProcessors == "false" {
		// Shared processor configuration
		fmt.Println("\n  Shared Processor Details:")
		fmt.Printf("    Desired Proc Units:    %s\n", profile.ProcessorConfig.SharedConfig.DesiredProcessingUnits)
		fmt.Printf("    Desired vCPUs:         %s\n", profile.ProcessorConfig.SharedConfig.DesiredVirtualProcessors)
		fmt.Printf("    Minimum Proc Units:    %s\n", profile.ProcessorConfig.SharedConfig.MinimumProcessingUnits)
		fmt.Printf("    Minimum vCPUs:         %s\n", profile.ProcessorConfig.SharedConfig.MinimumVirtualProcessors)
		fmt.Printf("    Maximum Proc Units:    %s\n", profile.ProcessorConfig.SharedConfig.MaximumProcessingUnits)
		fmt.Printf("    Maximum vCPUs:         %s\n", profile.ProcessorConfig.SharedConfig.MaximumVirtualProcessors)
		if profile.ProcessorConfig.SharedConfig.SharedProcessorPoolName != "" {
			fmt.Printf("    Processor Pool:        %s (ID: %s)\n", 
				profile.ProcessorConfig.SharedConfig.SharedProcessorPoolName,
				profile.ProcessorConfig.SharedConfig.SharedProcessorPoolID)
		}
		fmt.Printf("    Uncapped Weight:       %s\n", profile.ProcessorConfig.SharedConfig.UncappedWeight)
	} else {
		// Dedicated processor configuration
		fmt.Println("\n  Dedicated Processor Details:")
		fmt.Printf("    Desired Processors:    %s\n", profile.ProcessorConfig.DedicatedConfig.DesiredProcessors)
		fmt.Printf("    Minimum Processors:    %s\n", profile.ProcessorConfig.DedicatedConfig.MinimumProcessors)
		fmt.Printf("    Maximum Processors:    %s\n", profile.ProcessorConfig.DedicatedConfig.MaximumProcessors)
	}
	fmt.Println()
	
	// I/O Configuration
	fmt.Println("I/O Configuration:")
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Printf("  Max Virtual I/O Slots:   %s\n", profile.MaximumVirtualIOSlots)
	fmt.Println()
	
	// Additional Settings
	if profile.AffinityGroupID != "" || profile.DesiredProcessorCompatibilityMode != "" {
		fmt.Println("Additional Settings:")
		fmt.Println("─────────────────────────────────────────────────────────────")
		if profile.AffinityGroupID != "" {
			fmt.Printf("  Affinity Group ID:       %s\n", profile.AffinityGroupID)
		}
		if profile.DesiredProcessorCompatibilityMode != "" {
			fmt.Printf("  Processor Compat Mode:   %s\n", profile.DesiredProcessorCompatibilityMode)
		}
		if profile.ConnectionMonitoringEnabled != "" {
			fmt.Printf("  Connection Monitoring:   %s\n", profile.ConnectionMonitoringEnabled)
		}
		fmt.Println()
	}
	
	// Display full JSON for detailed inspection
	fmt.Println("Full Profile Data (JSON):")
	fmt.Println("─────────────────────────────────────────────────────────────")
	jsonData, err := json.MarshalIndent(profile, "  ", "  ")
	if err != nil {
		fmt.Printf("  Error marshaling to JSON: %v\n", err)
	} else {
		fmt.Printf("  %s\n", string(jsonData))
	}
	fmt.Println()

	fmt.Println("═════════════════════════════════════════════════════════════")
	fmt.Printf("✅ Profile '%s' retrieved successfully!\n", profile.ProfileName)
	fmt.Println("\n💡 API Efficiency Note:")
	fmt.Println("   This example uses GetLogicalPartitionProfile() which retrieves")
	fmt.Println("   a single profile by UUID - more efficient than GetLogicalPartitionProfiles()")
	fmt.Println("   when you only need one specific profile.")
}

// Made with Bob
