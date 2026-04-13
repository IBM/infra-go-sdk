package main

import (
	"flag"
	"fmt"
	"os"

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
	lparName := flag.String("lpar-name", "Go_LPAR_01", "Target LPAR Name")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	// Initialize CLI Logger
	cliLogger := hmc.NewDefaultLogger()
	cliLogger.SetPrefix("[CLI]")

	if *verbose {
		cliLogger.EnableDebug()
	} else {
		cliLogger.SetLevel(0) // Info level
	}

	if *password == "" || *sysName == "" || *lparName == "" {
		cliLogger.Fatal("Missing required arguments", "required", "hmc-pass, system-name, lpar-name")
	}

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if *verbose {
		restClient.EnableVerboseLogging()
	}

	if err := restClient.Login(*username, *password, *verbose); err != nil {
		cliLogger.Fatal("HMC Logon failed", "error", err)
	}
	defer restClient.Logoff()

	// Resolve System Name -> UUID
	cliLogger.Debug("Resolving System", "system", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		cliLogger.Fatal("Failed to resolve System", "error", err)
	}

	// Resolve LPAR Name -> UUID
	cliLogger.Debug("Resolving LPAR", "lpar", *lparName)
	_, lparUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		cliLogger.Fatal("Failed to resolve LPAR", "error", err)
	}

	// =========================================================================
	// FETCH AND DISPLAY ADAPTERS
	// =========================================================================
	adapters, err := restClient.GetVirtualFibreChannelClientAdapters(lparUUID, *verbose)
	if err != nil {
		cliLogger.Fatal("Failed to retrieve vFC adapters", "error", err)
	}

	if len(adapters) == 0 {
		cliLogger.Info("No Virtual Fibre Channel Adapters found for this LPAR.")
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