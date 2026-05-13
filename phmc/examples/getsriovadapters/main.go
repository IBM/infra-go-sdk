package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "LTC13U29-Ranier", "Managed System Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	flag.Parse()

	if *password == "" || *sysName == "" {
		log.Fatal("Error: hmc-pass and system-name are required.")
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

	// =========================================================================
	// FETCH & DISPLAY SR-IOV ADAPTERS
	// =========================================================================
	fmt.Println("\n📡 Discovering SR-IOV Adapters on Managed System...")
	fmt.Println("=========================================================================")

	adapters, err := restClient.GetSRIOVAdapters(context.Background(), sysUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch SR-IOV adapters: %v", err)
	}

	if len(adapters) == 0 {
		fmt.Printf("   ❌ No SR-IOV Adapters found on system '%s'.\n", *sysName)
	} else {
		fmt.Printf("✅ Found %d SR-IOV Adapter(s):\n\n", len(adapters))

		for _, adapter := range adapters {
			fmt.Printf("🏢 Adapter: %s\n", adapter.Description)
			fmt.Printf("   - Location: %s\n", adapter.LocationCode)
			fmt.Printf("   - Mode:     %s\n", adapter.AdapterMode)
			fmt.Printf("   - State:    %s\n", adapter.AdapterState)

			// If the adapter is in Dedicated mode, it won't expose physical SR-IOV ports to the API
			if len(adapter.PhysicalPorts) == 0 {
				if adapter.AdapterMode == "Dedicated" {
					fmt.Println("   ⚠️  Physical ports are hidden because the adapter is in 'Dedicated' mode.")
				} else {
					fmt.Println("   ⚠️  No physical ports configured or detected.")
				}
			} else {
				fmt.Println("   🔗 Physical Ports:")
				for _, port := range adapter.PhysicalPorts {
					// Handle cases where speed/capacity might not be populated yet
					speed := port.HardwareLinkSpeed
					if speed == "" {
						speed = "Unknown"
					}
					capacity := port.PortCapacity
					if capacity == "" {
						capacity = "0"
					}

					fmt.Printf("      * Port %s | Hardware Speed: %s | Total Capacity: %s%%\n",
						port.PortID, speed, capacity)
				}
			}
			fmt.Println("-------------------------------------------------------------------------")
		}
	}

	fmt.Println("🎉 SR-IOV Discovery Complete!")
	fmt.Println("=========================================================================")
}