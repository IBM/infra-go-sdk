package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	
	// Dynamic Target Identifiers
	sysName := flag.String("system-name", "", "Managed System Name")
	lparName := flag.String("lpar-name", "", "Target LPAR Name")
	
	// NPIV / vFC Specific Configuration
	viosName := flag.String("vios-name", "ltc09u31-vios1", "The name of the target VIOS")
	viosSlot := flag.Int("vios-slot", 10, "Virtual slot number on the VIOS (0 = Auto-Assign)")
	
	verbose := flag.Bool("verbose", true, "Enable verbose output (activates structured Debug logs)")
	flag.Parse()

	// =========================================================================
	// 1. INITIALIZE CLI LOGGER
	// =========================================================================
	// Create a logger for the main CLI application 
	cliLogger := hmc.NewDefaultLogger()
	cliLogger.SetPrefix("[CLI]")

	// If verbose is passed, turn on debug logging for BOTH the CLI and the SDK
	if *verbose {
		cliLogger.EnableDebug()
	} else {
		// Default to Info level for standard CLI output so Info messages show up
		cliLogger.SetLevel(0) // 0 is InfoLevel in charmbracelet/log
	}

	// Validation
	if *password == "" || *sysName == "" || *lparName == "" || *viosName == "" {
		cliLogger.Fatal("Missing required arguments", 
			"required", "hmc-pass, system-name, lpar-name, vios-name")
	}

	slotDisplay := "Auto-Assigned"
	if *viosSlot > 0 {
		slotDisplay = fmt.Sprintf("%d", *viosSlot)
	}

	cliLogger.Info("Provisioning Virtual Fibre Channel (vFC) Adapter", 
		"lpar", *lparName, 
		"vios_slot", slotDisplay,
	)

	// =========================================================================
	// 2. AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	
	// Sync the SDK logger level with the CLI logger
	if *verbose {
		restClient.EnableVerboseLogging()
	}

	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		cliLogger.Fatal("HMC Logon failed", "error", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// 3. DYNAMIC RESOLUTION (Name -> UUID & ID)
	// =========================================================================
	cliLogger.Debug("Resolving Managed System to UUID", "system", *sysName)
	
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		cliLogger.Fatal("Failed to resolve Managed System", "system", *sysName, "error", err)
	}

	cliLogger.Debug("Resolving LPAR to UUID", "lpar", *lparName)
	
	_, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		cliLogger.Fatal("Failed to resolve LPAR Name", "lpar", *lparName, "error", err)
	}
	cliLogger.Info("Target LPAR resolved", "uuid", lparUUID)

	cliLogger.Debug("Resolving VIOS to Partition ID", "vios", *viosName)
	
	viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID, *verbose)
	if err != nil {
		cliLogger.Fatal("Failed to fetch VIOS list", "error", err)
	}

	var targetViosID int
	for _, vios := range viosList {
		if strings.EqualFold(vios.PartitionName, *viosName) {
			targetViosID = vios.PartitionID
			break
		}
	}

	if targetViosID == 0 {
		cliLogger.Fatal("VIOS not found on system", "vios", *viosName, "system", *sysName)
	}
	cliLogger.Info("Target VIOS resolved", "id", targetViosID)

	// =========================================================================
	// 4. EXECUTE vFC CREATION
	// =========================================================================
	cliLogger.Info("Injecting vFC Client Adapter...")

	adapterUUID, err := restClient.CreateVirtualFibreChannelClientAdapter(lparUUID, targetViosID, *viosSlot, *verbose)
	if err != nil {
		cliLogger.Fatal("Failed to create vFC Client Adapter", "error", err)
	}

	cliLogger.Info("SUCCESS: Virtual Fibre Channel Adapter Provisioned!",
		"adapter_uuid", adapterUUID,
		"vios_name", *viosName,
		"vios_id", targetViosID,
		"vios_target_slot", slotDisplay,
	)
	
	cliLogger.Warn("Remember to map the server-side vFC adapter to a physical FC port (fcsX) on the VIOS!")
}