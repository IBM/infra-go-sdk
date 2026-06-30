package main

import (
	"log"
	"context"
	"flag"
	"fmt"
	"strings"

	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
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
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose

	// =========================================================================
	// 1. INITIALIZE CLI LOGGER
	// =========================================================================
	// Create a logger for the main CLI application 

	// If verbose is passed, turn on debug logging for BOTH the CLI and the SDK
	if *verbose {
	} else {
		// Default to Info level for standard CLI output so Info messages show up
		log.Printf(": %v", 0)
	}

	// Validation
	if *password == "" || *sysName == "" || *lparName == "" || *viosName == "" {
		log.Fatal("Missing required arguments")
	}

	slotDisplay := "Auto-Assigned"
	if *viosSlot > 0 {
		slotDisplay = fmt.Sprintf("%d", *viosSlot)
	}

	log.Println("Provisioning Virtual Fibre Channel (vFC")

	// =========================================================================
	// 2. AUTHENTICATION
	// =========================================================================
	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)
	
	// Sync the SDK logger level with the CLI logger

	if err := restClient.Login(context.Background(), *username, *password); err != nil {
		log.Fatal("HMC Logon failed")
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// 3. DYNAMIC RESOLUTION (Name -> UUID & ID)
	// =========================================================================
	log.Printf("Resolving Managed System to UUID: system=%v", *sysName)
	
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName)
	if err != nil || sysUUID == "" {
		log.Fatal("Failed to resolve Managed System")
	}

	log.Printf("Resolving LPAR to UUID: lpar=%v", *lparName)
	
	_, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName)
	if err != nil || lparUUID == "" {
		log.Fatal("Failed to resolve LPAR Name")
	}
	log.Printf("Target LPAR resolved: uuid=%v", lparUUID)

	log.Printf("Resolving VIOS to Partition ID: vios=%v", *viosName)
	
	viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID)
	if err != nil {
		log.Fatal("Failed to fetch VIOS list")
	}

	var targetViosID int
	for _, vios := range viosList {
		if strings.EqualFold(vios.PartitionName, *viosName) {
			targetViosID = vios.PartitionID
			break
		}
	}

	if targetViosID == 0 {
		log.Fatal("VIOS not found on system")
	}
	log.Printf("Target VIOS resolved: id=%v", targetViosID)

	// =========================================================================
	// 4. EXECUTE vFC CREATION
	// =========================================================================
	log.Println("Injecting vFC Client Adapter...")

	adapterUUID, err := restClient.CreateVirtualFibreChannelClientAdapter(lparUUID, targetViosID, *viosSlot)
	if err != nil {
		log.Fatal("Failed to create vFC Client Adapter")
	}

	log.Printf("[INFO] SUCCESS: Virtual Fibre Channel Adapter Provisioned! %v", "adapter_uuid", adapterUUID,
		"vios_name", *viosName,
		"vios_id", targetViosID,
		"vios_target_slot", slotDisplay,)
	
	log.Println("Remember to map the server-side vFC adapter to a physical FC port (fcsX")
}
