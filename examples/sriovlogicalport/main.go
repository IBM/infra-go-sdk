package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & GENERAL FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC13U29-Ranier", "Managed System Name")
	lparName := flag.String("lpar-name", "IMAGE_WORK-a9cbb4a2-00029acc", "Target LPAR Name")
	lparProfile := flag.String("lpar-profile", "default_profile", "Name of the LPAR profile to overwrite")
	forceSave := flag.Bool("force-save", true, "Force overwrite of the target profile")
	verbose := flag.Bool("verbose", false, "Enable verbose HTTP output")

	// --- CREATION FLAGS (Advanced Options) ---
	adapterID := flag.String("adapter-id", "1", "The Adapter ID of the backing SR-IOV adapter (Create Mode)")
	portID := flag.String("port-id", "0", "The Physical Port ID on the adapter (Create Mode)")
	capacity := flag.String("capacity", "2.0%", "Configured capacity percentage (Create Mode)")
	promiscuous := flag.Bool("promiscuous", false, "Enable Promiscuous Mode on the port (Create Mode)")
	portVLAN := flag.String("vlan-id", "0", "The primary Port VLAN ID (Create Mode)")
	allowedMACs := flag.String("allowed-macs", "ALL", "Allowed MAC Addresses: ALL, NONE, or specify (Create Mode)")
	allowedVLANs := flag.String("allowed-vlans", "ALL", "Allowed VLANs: ALL, NONE, or specify (Create Mode)")
	allowedPriorities := flag.String("allowed-priorities", "0", "Allowed 802.1Q Priorities (Create Mode)")

	// --- DELETION & LISTING FLAGS ---
	deleteMode := flag.Bool("delete", false, "Set to true to DELETE SR-IOV Logical Ports")
	listMode := flag.Bool("list", false, "Set to true to purely LIST the current SR-IOV Logical Ports")
	logicalPortsRaw := flag.String("logical-ports", "", "Comma-separated list of Logical Port IDs or Location Codes to delete")

	flag.Parse()

	// --- Validation ---
	if *password == "" || *sysName == "" || *lparName == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, and lpar-name are required.")
	}

	if *deleteMode && *logicalPortsRaw == "" {
		log.Fatal("❌ Error: When using -delete, you must provide targets using -logical-ports.")
	}

	if *deleteMode && *listMode {
		log.Fatal("❌ Error: Cannot use -delete and -list at the same time.")
	}

	// =========================================================================
	// AUTHENTICATION & SYSTEM RESOLUTION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	fmt.Printf("Resolving System UUID for '%s'...\n", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	fmt.Printf("Resolving LPAR UUID for '%s'...\n", *lparName)
	_, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}

	// =========================================================================
	// READ-ONLY: LIST MODE
	// =========================================================================
	if *listMode {
		fmt.Printf("\n📡 LISTING SR-IOV Logical Ports for LPAR '%s'...\n", *lparName)
		fmt.Println("=========================================================================")

		logicalPorts, err := restClient.GetSRIOVLogicalPorts(context.Background(), lparUUID, *verbose)
		if err != nil {
			log.Fatalf("❌ Failed to fetch SR-IOV Logical Ports: %v", err)
		}

		if len(logicalPorts) == 0 {
			fmt.Println("   ❌ No SR-IOV Logical Ports found on this LPAR.")
		} else {
			fmt.Printf("✅ Found %d SR-IOV Logical Port(s):\n\n", len(logicalPorts))
			for _, port := range logicalPorts {
				fmt.Printf("   🔌 Logical Port ID: %s (UUID: %s)\n", port.LogicalPortID, port.UUID)
				fmt.Printf("      - Location:      %s\n", port.LocationCode)
				fmt.Printf("      - Config ID:     %s\n", port.ConfigurationID)
				fmt.Printf("      - Adapter ID:    %s\n", port.AdapterID)
				fmt.Printf("      - Physical ID:   %s\n", port.PhysicalPortID)
				fmt.Printf("      - Capacity:      %s\n", port.ConfiguredCapacity)
				fmt.Printf("      - VLAN ID:       %s\n", port.PortVLANID)
				fmt.Printf("      - Promiscuous:   %t\n", port.IsPromiscuous)
				fmt.Printf("      - Functional:    %t\n", port.IsFunctional)
				fmt.Println("-------------------------------------------------------------------------")
			}
		}
		// Exit early for read-only mode so we don't dump XML
		return
	}

	// =========================================================================
	// DIRECTORY & FILENAME PREPARATION (For Mutations)
	// =========================================================================
	outDir := "out"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("❌ Failed to create output directory: %v", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	beforeFile := fmt.Sprintf("%s/lpar_before_%s_%s.xml", outDir, *lparName, timestamp)
	afterFile := fmt.Sprintf("%s/lpar_after_%s_%s.xml", outDir, *lparName, timestamp)

	// =========================================================================
	// DUMP "BEFORE" XML
	// =========================================================================
	fmt.Println("\n[Diff Tool] Fetching 'BEFORE' XML state...")
	beforeXML, err := restClient.GetRawLparXML(sysUUID, lparUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch BEFORE XML: %v", err)
	}
	if err := os.WriteFile(beforeFile, []byte(beforeXML), 0644); err != nil {
		log.Fatalf("❌ Failed to write BEFORE file: %v", err)
	}
	fmt.Printf("   -> Saved '%s'\n", beforeFile)

	// =========================================================================
	// EXECUTE THE OPERATION (CREATE OR DELETE)
	// =========================================================================
	var operationStatus string
	if *deleteMode {
		fmt.Println("\n🗑️  DELETING SR-IOV Logical Ports...")
		fmt.Println("=========================================================================")
		fmt.Printf("Target LPAR: %s\n", *lparName)
		fmt.Printf("Targets:     %s\n", *logicalPortsRaw)
		fmt.Println("-------------------------------------------------------------------------")

		// 1. Split the comma-separated string into a slice
		targets := strings.Split(*logicalPortsRaw, ",")
		var cleanTargets []string
		for _, t := range targets {
			if strings.TrimSpace(t) != "" {
				cleanTargets = append(cleanTargets, strings.TrimSpace(t))
			}
		}

		if len(cleanTargets) == 0 {
			log.Fatalf("❌ Error: No valid targets provided for deletion.")
		}

		// 2. Call the Smart SDK Deleter
		err = restClient.DeleteSRIOVLogicalPorts(context.Background(), lparUUID, cleanTargets, *verbose)
		if err != nil {
			log.Fatalf("❌ Deletion Failed: %v", err)
		}
		
		fmt.Printf("✅ Successfully deleted %d SR-IOV Logical Port target(s)!\n", len(cleanTargets))

	} else {

		// DEFAULT TO CREATE MODE
		fmt.Println("\n📡 PROVISIONING SR-IOV Logical Port...")
		fmt.Println("=========================================================================")
		fmt.Printf("Target LPAR:   %s\n", *lparName)
		fmt.Printf("Adapter/Port:  Adapter %s | Port %s\n", *adapterID, *portID)
		fmt.Printf("Capacity:      %s\n", *capacity)
		fmt.Printf("Port VLAN ID:  %s\n", *portVLAN)
		fmt.Printf("Promiscuous:   %t\n", *promiscuous)
		fmt.Printf("Allowed MACs:  %s\n", *allowedMACs)
		fmt.Printf("Allowed VLANs: %s\n", *allowedVLANs)
		fmt.Println("-------------------------------------------------------------------------")

		// Populate the advanced options struct
		opts := hmc.SRIOVPortCreateOptions{
			Capacity:               *capacity,
			PortVLANID:             *portVLAN,
			IsPromiscuous:          *promiscuous,
			AllowedMACAddresses:    *allowedMACs,
			AllowedVLANs:           *allowedVLANs,
			Allowed8021QPriorities: *allowedPriorities,
		}

		operationStatus,err = restClient.CreateSRIOVLogicalPort(lparUUID, *adapterID, *portID, opts, *verbose)
		if err != nil {
			log.Fatalf("❌ Provisioning Failed: %v", err)
		}
		fmt.Println("✅ SR-IOV Logical Port Provisioned Successfully!")
	}
	// =========================================================================
	// 5. SAVE LPAR PROFILE (Only if topology changes were made)
	// =========================================================================
	if operationStatus == "SUCCESS" || operationStatus == "SUCCESS_WITH_RMC_WARNING" {
		fmt.Printf("\n[Profile] Saving running configuration to LPAR profile '%s'...\n", *lparProfile)
		saveErr := restClient.SaveCurrentLparConfig(context.Background(), lparUUID, *lparProfile, *forceSave, *verbose)
		if saveErr != nil {
			log.Printf("⚠️ Warning: vFC topology modified dynamically, but failed to save LPAR profile: %v\n", saveErr)
		} else {
			fmt.Println("✅ Success: LPAR profile saved. The Client Fibre Channel adapters will persist across reboots.")
		}
	} else {
		fmt.Printf("\n[Profile] No architectural changes were made to the LPAR. Profile save skipped.\n")
	}
	// =========================================================================
	// DUMP "AFTER" XML
	// =========================================================================
	fmt.Println("\n[Diff Tool] Fetching 'AFTER' XML state...")
	afterXML, err := restClient.GetRawLparXML(sysUUID, lparUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch AFTER XML: %v", err)
	}
	if err := os.WriteFile(afterFile, []byte(afterXML), 0644); err != nil {
		log.Fatalf("❌ Failed to write AFTER file: %v", err)
	}
	fmt.Printf("   -> Saved '%s'\n", afterFile)

	fmt.Println("=========================================================================")
	fmt.Printf("🎉 OPERATION COMPLETE! You can now diff the XML files:\n")
	fmt.Printf("   code --diff %s %s\n", beforeFile, afterFile)
	fmt.Println("=========================================================================")
}