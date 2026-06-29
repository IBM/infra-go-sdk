package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	hmc "github.com/IBM/infra-go-sdk/phmc"
)

func main() {
	// NOTE:
	// This example exercises GetNetworkBootDevices for a VIOS profile using the
	// documented VirtualIOServer job flow. In validation against tested HMCs,
	// the request is accepted only up to job submission and then fails with
	// FAILED_TO_START because the HMC reports that
	// LogicalPartitionProfileUUID is not allowed for this job, even though IBM
	// documentation shows that parameter in the sample request.
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	viosName := flag.String("vios-name", "", "Target VIOS Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()
	_ = verbose

	if *password == "" || *sysName == "" || *viosName == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, and vios-name are required")
	}

	fmt.Println("=================================================================")
	fmt.Println("  VIOS Network Boot Device Information Utility")
	fmt.Println("=================================================================")
	fmt.Printf("HMC:    %s\n", *hmcIP)
	fmt.Printf("System: %s\n", *sysName)
	fmt.Printf("VIOS:   %s\n", *viosName)
	fmt.Println("=================================================================")

	// 1. Initialize & Login
	fmt.Println("\nStep 1: Connecting to HMC...")
	client := hmc.NewRestClient(*hmcIP)
	if err := client.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ Login failed: %v", err)
	}
	defer client.Logoff(context.Background())
	fmt.Println("✅ Connected to HMC")

	// 2. Resolve System UUID
	fmt.Printf("Step 2: Resolving System UUID for '%s'...\n", *sysName)
	_, sysUUID, err := client.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ Managed System '%s' not found: %v", *sysName, err)
	}
	fmt.Printf("✅ System UUID: %s\n", sysUUID)

	// 3. Resolve VIOS UUID
	fmt.Printf("Step 3: Resolving VIOS UUID for '%s'...\n", *viosName)
	viosUUID, err := hmc.GetViosID(context.Background(), client, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found on system '%s': %v", *viosName, *sysName, err)
	}
	fmt.Printf("✅ VIOS UUID: %s\n", viosUUID)

	// 4. Fetch the Comprehensive VIOS Details to extract the Profile UUID
	fmt.Println("Step 4: Fetching detailed VIOS configuration to extract Profile UUID...")
	viosDetailed, err := client.GetVirtualIOServer(context.Background(), viosUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get VIOS details: %v", err)
	}
    
	if viosDetailed.PartitionState == "running" {
		fmt.Println("\n⚠️  WARNING: VIOS is currently RUNNING")
		fmt.Println("   The GetNetworkBootDevices job requires the VIOS to be powered off first.")
		fmt.Println("   Exiting to prevent destructive restart.")
		return
	}

	profileHref := viosDetailed.AssociatedPartitionProfile.Href
	var profileUUID string
	if profileHref != "" {
		profileUUID = profileHref[len(profileHref)-36:]
		fmt.Printf("✅ Associated Profile UUID: %s\n", profileUUID)
	} else {
		log.Fatal("❌ No associated partition profile found for this VIOS - cannot retrieve network boot devices")
	}

	// 5. Fetch Network Boot Devices from profile
	fmt.Println("\nStep 5: Firing GetNetworkBootDevices Job for VIOS profile...")
	bootDevices, err := client.GetNetworkBootDevicesForVios(context.Background(), viosUUID, profileUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ GetNetworkBootDevices failed: %v", err)
	}

	if len(bootDevices) == 0 {
		fmt.Printf("⚠️  No network boot devices found in profile for VIOS '%s'\n", *viosName)
		return
	}

	fmt.Printf("✅ Retrieved %d network boot device(s) from profile\n", len(bootDevices))

	// 6. Display Network Boot Device Information
	fmt.Println("\n=================================================================")
	fmt.Println("  VIOS NETWORK BOOT DEVICE INFORMATION")
	fmt.Println("=================================================================")

	for i, device := range bootDevices {
		fmt.Printf("--- Boot Device %d ---\n", i+1)
		fmt.Printf("Device Name:         %s\n", device.DeviceName)
		fmt.Printf("Device Type:         %s\n", device.DeviceType)
		fmt.Printf("Location Code:       %s\n", device.LocationCode)
		fmt.Printf("MAC Address:         %s\n", device.MACAddress)
		fmt.Println(strings.Repeat("-", 65))
	}
}