package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	hmc "github.com/sudeeshjohn/PowerHMC" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *sysName == "" {
		log.Fatal("Error: hmc-pass and system-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("❌ HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found: %v", *sysName, err)
	}

	// =========================================================================
	// FETCH COMPREHENSIVE DETAILS
	// =========================================================================
	detailedSystem, err := restClient.GetManagedSystem(sysUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch detailed system info: %v", err)
	}

	// =========================================================================
	// DISPLAY COMPLETE PHYSICAL INVENTORY
	// =========================================================================
	fmt.Printf("\n✅ Full Physical I/O Slot Inventory for system '%s':\n", *sysName)
	fmt.Println("======================================================================================================================")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "DEVICE NAME (LOC CODE)\tASSIGNABLE\tATTACHED PARTITION\tSTATUS / DESCRIPTION")
	fmt.Fprintln(w, "----------------------\t----------\t------------------\t--------------------")

	totalDevices := 0

	// We iterate through all Physical PCIe Slots on the Buses (This guarantees we see empty slots too)
	for _, bus := range detailedSystem.IOConfig.IOBuses {
		for _, slot := range bus.IOSlots {
			totalDevices++
			adapter := slot.RelatedIOAdapter

			// 1. Determine Assignability
			assignable := "No"
			if adapter.LogicalPartitionAssignmentCapable {
				assignable = "Yes"
			}

			// 2. Determine Description
			desc := adapter.Description
			if desc == "" || desc == "Empty slot" {
				desc = slot.Description // Fallback to slot description
			}

			// 3. Determine Location Code
			locCode := adapter.DeviceName
			if locCode == "" {
				locCode = slot.PhysicalLocationCode
			}

			// 4. Determine Attached Partition
			attached := slot.PartitionName
			if attached == "" {
				if desc == "Empty slot" {
					attached = "-"
				} else {
					attached = "Unassigned / Hypervisor"
				}
			} else {
				// Format as "Name (ID)"
				attached = fmt.Sprintf("%s (%d)", slot.PartitionName, slot.PartitionID)
			}

			// Print the row
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", 
				locCode, 
				assignable, 
				attached, 
				desc,
			)
		}
	}
	
	w.Flush()
	fmt.Println("======================================================================================================================")
	fmt.Printf("Total Physical Slots Scanned: %d\n", totalDevices)
}