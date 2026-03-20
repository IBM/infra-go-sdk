package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/PowerHMC" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	
	// We take the human-readable names now instead of the UUID!
	sysName := flag.String("system-name", "LTC13U05", "Managed System Name")
	lparName := flag.String("lpar-name", "prow-3e3e-bas-875d8322-00003bfc", "LPAR Name")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *sysName == "" || *lparName == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, and lpar-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff()

	// =========================================================================
	// RESOLUTION: NAME -> UUID
	// =========================================================================
	fmt.Printf("Resolving System '%s' to UUID...\n", *sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ Failed to resolve Managed System: %v", err)
	}

	fmt.Printf("Resolving LPAR '%s' to UUID...\n", *lparName)
	lparUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ Failed to resolve LPAR Name '%s'. Does it exist on system '%s'?\n", *lparName, *sysName)
	}

	// =========================================================================
	// FETCH EXHAUSTIVE DETAILS
	// =========================================================================
	fmt.Printf("Fetching exhaustive details for LPAR UUID: %s...\n", lparUUID)
	
	lpar, err := restClient.GetLogicalPartitionDetailed(lparUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Error: %v", err)
	}

	// Output the entire struct as pretty JSON
	prettyJSON, _ := json.MarshalIndent(lpar, "", "    ")
	fmt.Println(string(prettyJSON))

	// Example of targeted access
	fmt.Println("\n=======================================================")
	fmt.Printf("Partition Name:       %s\n", lpar.PartitionName)
	fmt.Printf("State:                %s (RMC: %s)\n", lpar.PartitionState, lpar.ResourceMonitoringControlState)
	
	// FIXED: Using %.0f to cleanly print the float64 memory value
	fmt.Printf("Current Memory:       %.0f MB\n", lpar.PartitionMemoryConfiguration.CurrentMemory)
	
	fmt.Printf("Current Processing:   %.1f Units across %d vCPUs\n", 
		lpar.PartitionProcessorConfiguration.CurrentSharedProcessorConfiguration.CurrentProcessingUnits,
		lpar.PartitionProcessorConfiguration.CurrentSharedProcessorConfiguration.AllocatedVirtualProcessors,
	)
	fmt.Printf("Boot String:          %s\n", lpar.BootListInformation.BootDeviceList)
	fmt.Printf("Attached vSCSI Adapters: %d\n", len(lpar.VirtualSCSIClientAdapters))
	fmt.Println("=======================================================")
}