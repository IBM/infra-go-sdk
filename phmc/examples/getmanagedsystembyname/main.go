package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// Command-line flags
	hmcIP := flag.String("hmc", "192.0.2.1", "HMC IP address")
	username := flag.String("user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("pass", "REDACTED_HMC_PASS<==", "HMC password")
	targetName := flag.String("system", "LTC09U31-ZZ", "Managed system name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	
	flag.Parse()

	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	fmt.Printf("Searching for Managed System: %s...\n", *targetName)
	
	// Use the upgraded function that returns the UUID and our comprehensive struct
	uuid, detailedSystem, err := restClient.GetManagedSystemByName(context.Background(), *targetName, *verbose)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	if detailedSystem == nil {
		fmt.Println("System not found.")
		return
	}

	// =========================================================================
	// SUCCESSFUL EXTRACTION - No more etree element hunting!
	// =========================================================================
	fmt.Println("\n--- System Details ---")
	fmt.Printf("UUID:           %s\n", uuid)
	fmt.Printf("System Name:    %s\n", detailedSystem.SystemName)
	fmt.Printf("State:          %s\n", detailedSystem.State)
	
	// Format the MTMS exactly as IBM displays it: Type-Model*Serial
	if detailedSystem.MTMS.MachineType != "" {
		fmt.Printf("MTMS:           %s-%s*%s\n", 
			detailedSystem.MTMS.MachineType, 
			detailedSystem.MTMS.Model, 
			detailedSystem.MTMS.SerialNumber,
		)
	}
	
	fmt.Printf("Firmware:       %s\n", detailedSystem.SystemFirmware)
	fmt.Printf("Max Partitions: %.0f\n", detailedSystem.MaximumPartitions)

	// Bonus: Look how easy it is to grab deep configuration data now!
	fmt.Println("\n--- Quick Specs ---")
	fmt.Printf("IP Address:     %s\n", detailedSystem.PrimaryIPAddress)
	fmt.Printf("Total Memory:   %.0f MB\n", detailedSystem.MemoryConfig.InstalledSystemMemory)
	fmt.Printf("Total CPUs:     %.1f Cores\n", detailedSystem.ProcessorConfig.InstalledSystemProcessorUnits)
	fmt.Printf("WWPN Prefix:    %s\n", detailedSystem.IOConfig.WWPNPrefix)
}