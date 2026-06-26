package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address (Required)")
	username := flag.String("hmc-user", "", "HMC username (Required)")
	password := flag.String("hmc-pass", "", "HMC password (Required)")
	
	// We take the human-readable names now instead of the UUID!
	sysName := flag.String("system-name", "", "Managed System Name (Required)")
	lparName := flag.String("lpar-name", "", "LPAR Name (Required)")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()
	_ = verbose

	if *hmcIP == "" || *username == "" || *password == "" || *sysName == "" || *lparName == "" {
		log.Fatal("❌ Error: hmc-ip, hmc-user, hmc-pass, system-name, and lpar-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// RESOLUTION: NAME -> UUID
	// =========================================================================
	fmt.Printf("Resolving System '%s' to UUID...\n", *sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ Failed to resolve Managed System: %v", err)
	}

	fmt.Printf("Resolving LPAR '%s' to UUID...\n", *lparName)
	_,lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ Failed to resolve LPAR Name '%s'. Does it exist on system '%s'?\n", *lparName, *sysName)
	}

	// =========================================================================
	// FETCH EXHAUSTIVE DETAILS
	// =========================================================================
	fmt.Printf("Fetching exhaustive details for LPAR UUID: %s...\n", lparUUID)
	
	lpar, err := restClient.GetLogicalPartitionDetailed(context.Background(), lparUUID, *verbose)
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
	
	fmt.Printf("Current Processing:   %.1f Units across %f vCPUs\n", 
		lpar.PartitionProcessorConfiguration.CurrentSharedProcessorConfiguration.CurrentProcessingUnits,
		lpar.PartitionProcessorConfiguration.CurrentSharedProcessorConfiguration.AllocatedVirtualProcessors,
	)
	fmt.Printf("Boot String:          %s\n", lpar.BootListInformation.BootDeviceList)
	fmt.Printf("Attached vSCSI Adapters: %d\n", len(lpar.VirtualSCSIClientAdapters))
	fmt.Println("=======================================================")
}