package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC13U29-Ranier", "Managed System Name")
	lparName := flag.String("lpar-name", "IMAGE_WORK-a9cbb4a2-00029acc", "Target LPAR Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	flag.Parse()

	if *password == "" || *sysName == "" || *lparName == "" {
		log.Fatal("Error: hmc-pass, system-name, and lpar-name are required.")
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
	// 1. FETCH DEDICATED VIRTUAL NICS (vNIC)
	// =========================================================================
	fmt.Printf("\n📡 Discovering Dedicated Virtual NICs on LPAR '%s'...\n", *lparName)
	fmt.Println("=========================================================================")

	vnics, err := restClient.GetDedicatedVirtualNICs(context.Background(), lparUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch Dedicated vNICs: %v", err)
	}

	if len(vnics) == 0 {
		fmt.Println("   ❌ No Dedicated Virtual NICs found on this LPAR.")
	} else {
		fmt.Printf("✅ Found %d Dedicated Virtual NIC(s):\n\n", len(vnics))
		for _, vnic := range vnics {
			fmt.Printf("   🌐 vNIC Slot: %-4s | MAC: %s\n", vnic.VirtualSlotNumber, vnic.MACAddress)
			fmt.Printf("      - Capacity:  %s%%\n", vnic.Capacity)
			fmt.Printf("      - Varied On: %s\n", vnic.VariedOn)
			
			// Display the HATEOAS Link referencing the SR-IOV port it uses
			if vnic.BackingLogicalPort.Href != "" {
				fmt.Printf("      - Backing:   %s\n", vnic.BackingLogicalPort.Href)
			}
			fmt.Println("-------------------------------------------------------------------------")
		}
	}

	// =========================================================================
	// 2. FETCH SR-IOV LOGICAL PORTS
	// =========================================================================
	fmt.Printf("\n📡 Discovering Underlying SR-IOV Logical Ports on LPAR '%s'...\n", *lparName)
	fmt.Println("=========================================================================")

	logicalPorts, err := restClient.GetSRIOVLogicalPorts(context.Background(), lparUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch SR-IOV Logical Ports: %v", err)
	}

	if len(logicalPorts) == 0 {
		fmt.Println("   ❌ No SR-IOV Logical Ports found provisioned to this LPAR.")
	} else {
		fmt.Printf("✅ Found %d SR-IOV Logical Port(s):\n\n", len(logicalPorts))
		for _, port := range logicalPorts {
			fmt.Printf("   🔌 Logical Port ID: %s\n", port.LogicalPortID)
			fmt.Printf("      - Location:     %s\n", port.LocationCode)
			fmt.Printf("      - Adapter ID:   %s\n", port.AdapterID)
			fmt.Printf("      - Physical ID:  %s\n", port.PhysicalPortID)
			fmt.Printf("      - Capacity:     %s%%\n", port.ConfiguredCapacity)
			fmt.Printf("      - VLAN ID:      %s\n", port.PortVLANID)
			fmt.Printf("      - Promiscuous:  %t\n", port.IsPromiscuous)
			fmt.Printf("      - Functional:   %t\n", port.IsFunctional)
			fmt.Println("-------------------------------------------------------------------------")
		}
	}

	fmt.Println("\n🎉 SR-IOV Virtual Networking Discovery Complete!")
	fmt.Println("=========================================================================")
}