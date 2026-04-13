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
	
	// Changed to handle multiple ports and default to empty for mapping ALL discovered ports
	fcPortsRaw := flag.String("fc-ports", "", "Comma-separated list of physical FC ports (e.g., 'fcs0,fcs1'). Leave empty to auto-select ALL available ports on the VIOS.")
	
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

	if *password == "" || *sysName == "" || *viosName == "" || *lparName == "" {
		cliLogger.Fatal("Missing required arguments", "required", "hmc-pass, system-name, vios-name, lpar-name")
	}

	cliLogger.Info("Preparing to create NPIV (vFC) Mappings", 
		"vios", *viosName, 
		"lpar", *lparName,
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
	// 2. DYNAMIC FIBRE CHANNEL PORT DISCOVERY & VALIDATION
	// =========================================================================
	cliLogger.Debug("Fetching detailed VIOS info to discover physical FC ports", "viosUUID", viosUUID)
	
	viosDetails, err := restClient.GetVirtualIOServer(viosUUID, *verbose)
	if err != nil {
		cliLogger.Fatal("Failed to fetch detailed VIOS configuration", "error", err)
	}

	var availablePorts []string
	
	// Iterate through the exhaustive struct to find the physical FC adapters and their ports
	for _, profileSlot := range viosDetails.PartitionIOConfiguration.ProfileIOSlots {
		fcAdapter := profileSlot.AssociatedIOSlot.RelatedIOAdapter.PhysicalFibreChannelAdapter
		if len(fcAdapter.PhysicalFibreChannelPorts) > 0 {
			for _, port := range fcAdapter.PhysicalFibreChannelPorts {
				if port.PortName != "" {
					availablePorts = append(availablePorts, port.PortName)
					cliLogger.Debug("Discovered Physical FC Port", "portName", port.PortName, "wwpn", port.WWPN)
				}
			}
		}
	}

	if len(availablePorts) == 0 {
		cliLogger.Fatal("No Physical Fibre Channel ports found on this VIOS. NPIV mapping cannot proceed.", "vios", *viosName)
	}

	var selectedPorts []string

	// Determine which ports to use
	if *fcPortsRaw == "" {
		// Auto-select ALL available ports for HA
		selectedPorts = availablePorts
		cliLogger.Info("Auto-selected ALL discovered Physical FC Ports", "ports", selectedPorts)
	} else {
		// Validate that the user-provided ports actually exist on this VIOS
		rawPorts := strings.Split(*fcPortsRaw, ",")
		for _, rawPort := range rawPorts {
			cleanPort := strings.TrimSpace(rawPort)
			if cleanPort == "" {
				continue
			}

			found := false
			for _, ap := range availablePorts {
				if strings.EqualFold(ap, cleanPort) {
					found = true
					break
				}
			}

			if !found {
				cliLogger.Fatal("Requested FC port not found on VIOS", 
					"requested", cleanPort, 
					"available_ports", availablePorts,
				)
			}
			selectedPorts = append(selectedPorts, cleanPort)
		}
		cliLogger.Info("Validated requested FC Ports exist on VIOS", "ports", selectedPorts)
	}

	if len(selectedPorts) == 0 {
		cliLogger.Fatal("No valid Fibre Channel ports selected for mapping.")
	}

	// =========================================================================
	// 3. EXECUTE MAPPING
	// =========================================================================
	fmt.Printf("\n🚀 Mapping LPAR '%s' directly to Physical FC Ports %v on VIOS '%s'...\n", *lparName, selectedPorts, *viosName)

	status, err := restClient.CreateVirtualFibreChannelMappings(sysUUID, viosUUID, lparUUID, selectedPorts, *verbose)
	if err != nil {
		cliLogger.Fatal("Failed to create vFC Mappings", "error", err)
	}

	fmt.Println("\n=========================================================================")
	fmt.Printf(" 🎉 SUCCESS: NPIV Topology Provisioned!\n")
	fmt.Printf("    Status: %s\n", status)
	fmt.Printf("    Mapped Ports: %v\n", selectedPorts)
	fmt.Println("    The HMC has automatically generated the Virtual Client and Server adapters.")
	fmt.Println("=========================================================================")
}