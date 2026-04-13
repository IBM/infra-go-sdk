package main

import (
	"flag"
	"fmt"
	"strings"

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
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS Name")
	
	// Accepts multiple ports so you can tear down a dual-fabric configuration in one pass
	fcPortsRaw := flag.String("fc-ports", "fcs1", "Comma-separated list of physical FC ports to unmap (e.g., 'fcs0,fcs1')")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output (activates structured Debug logs)")
	flag.Parse()

	// Initialize CLI Logger
	cliLogger := hmc.NewDefaultLogger()
	cliLogger.SetPrefix("[CLI]")

	if *verbose {
		cliLogger.EnableDebug()
	} else {
		cliLogger.SetLevel(0) // Info level
	}

	if *password == "" || *sysName == "" || *viosName == "" || *lparName == "" || *fcPortsRaw == "" {
		cliLogger.Fatal("Missing required arguments", "required", "hmc-pass, system-name, vios-name, lpar-name, fc-ports")
	}

	// Clean and parse the comma-separated ports
	rawPorts := strings.Split(*fcPortsRaw, ",")
	var portsToDelete []string
	for _, p := range rawPorts {
		cleanPort := strings.TrimSpace(p)
		if cleanPort != "" {
			portsToDelete = append(portsToDelete, cleanPort)
		}
	}

	if len(portsToDelete) == 0 {
		cliLogger.Fatal("No valid Fibre Channel ports provided to delete")
	}

	cliLogger.Info("Preparing to delete NPIV (vFC) Mappings", 
		"vios", *viosName, 
		"lpar", *lparName, 
		"ports_count", len(portsToDelete),
		"ports", portsToDelete,
	)

	// =========================================================================
	// 1. AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if *verbose {
		restClient.EnableVerboseLogging()
	}

	if err := restClient.Login(*username, *password, *verbose); err != nil {
		cliLogger.Fatal("HMC Logon failed", "error", err)
	}
	defer restClient.Logoff()

	// Resolve System
	cliLogger.Debug("Resolving System", "system", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		cliLogger.Fatal("Failed to resolve System", "error", err)
	}

	// Resolve LPAR
	cliLogger.Debug("Resolving LPAR", "lpar", *lparName)
	_, lparUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		cliLogger.Fatal("Failed to resolve LPAR", "error", err)
	}

	// Resolve VIOS
	cliLogger.Debug("Resolving VIOS", "vios", *viosName)
	viosUUID, err := hmc.GetViosID(restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		cliLogger.Fatal("VIOS not found on system", "vios", *viosName)
	}

	// =========================================================================
	// 2. EXECUTE MAPPING DELETION
	// =========================================================================
	fmt.Printf("\n🗑️  Removing %d NPIV mappings from LPAR '%s' on VIOS '%s'...\n", len(portsToDelete), *lparName, *viosName)

	status, err := restClient.DeleteVirtualFibreChannelMappings(sysUUID, viosUUID, lparUUID, portsToDelete, *verbose)
	if err != nil {
		cliLogger.Fatal("Failed to delete vFC Mappings", "error", err)
	}

	fmt.Println("\n=========================================================================")
	if status == "NOT_FOUND" {
		fmt.Printf(" ℹ️  Idempotent Success: No mappings found for ports %v on this LPAR.\n", portsToDelete)
	} else if status == "SUCCESS_WITH_RMC_WARNING" {
		fmt.Printf(" ⚠️  SUCCESS (With RMC Warning): Mappings were removed from the HMC configuration,\n")
		fmt.Printf("    but the dynamic push to the OS timed out. This is expected if the LPAR is powered off.\n")
	} else {
		fmt.Printf(" 🎉 SUCCESS: %d NPIV mapping(s) successfully unmapped and destroyed!\n", len(portsToDelete))
	}
	fmt.Println("=========================================================================")
}