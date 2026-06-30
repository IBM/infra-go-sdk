package main

import (
	"log"
	"context"
	"flag"
	"fmt"
	"os"

	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	
	sysName := flag.String("system-name", "", "Managed System Name")
	lparName := flag.String("lpar-name", "", "Target LPAR Name")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose

	// Initialize CLI Logger


	if *password == "" || *sysName == "" || *lparName == "" {
		log.Fatal("Missing required arguments")
	}

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)

	if err := restClient.Login(context.Background(), *username, *password); err != nil {
		log.Fatal("HMC Logon failed")
	}
	defer restClient.Logoff(context.Background())

	// Resolve System Name -> UUID
	log.Printf("Resolving System: system=%v", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName)
	if err != nil || sysUUID == "" {
		log.Fatal("Failed to resolve System")
	}

	// Resolve LPAR Name -> UUID
	log.Printf("Resolving LPAR: lpar=%v", *lparName)
	_, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName)
	if err != nil || lparUUID == "" {
		log.Fatal("Failed to resolve LPAR")
	}

	// =========================================================================
	// FETCH AND DISPLAY ADAPTERS
	// =========================================================================
	adapters, err := restClient.GetVirtualFibreChannelClientAdapters(lparUUID)
	if err != nil {
		log.Fatal("Failed to retrieve vFC adapters")
	}

	if len(adapters) == 0 {
		log.Println("No Virtual Fibre Channel Adapters found for this LPAR.")
		os.Exit(0)
	}

	fmt.Printf("\nFound %d Virtual Fibre Channel Adapter(s) on %s:\n", len(adapters), *lparName)

	// Mimic the Python output formatting
	for _, adapter := range adapters {
		fmt.Println("\n")
		fmt.Printf("%-35s : %s\n", "LocalPartitionID", adapter.LocalPartitionID)
		fmt.Printf("%-35s : %s\n", "VirtualSlotNumber", adapter.VirtualSlotNumber)
		fmt.Printf("%-35s : %s\n", "RequiredAdapter", adapter.RequiredAdapter)
		fmt.Printf("%-35s : %s\n", "ConnectingPartitionID", adapter.ConnectingPartitionID)
		fmt.Printf("%-35s : %s\n", "ConnectingVirtualSlotNumber", adapter.ConnectingVirtualSlotNumber)
		fmt.Printf("%-35s : %s\n", "WWPNs", adapter.WWPNs)
	}
	fmt.Println()
}
