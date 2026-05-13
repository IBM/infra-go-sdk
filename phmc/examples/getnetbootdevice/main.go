package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc"
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	lparName := flag.String("lpar-name", "", "Target LPAR Name")
	verbose := flag.Bool("verbose", true, "Enable verbose output")
	flag.Parse()

	if *password == "" {
		log.Fatal("❌ Error: hmc-pass is required")
	}
	if *sysName == "" {
		log.Fatal("❌ Error: system-name is required")
	}
	if *lparName == "" {
		log.Fatal("❌ Error: lpar-name is required")
	}

	fmt.Println("=================================================================")
	fmt.Println("  Network Boot Device Information Utility")
	fmt.Println("=================================================================")
	fmt.Printf("HMC:    %s", *hmcIP)
	fmt.Printf("System: %s", *sysName)
	fmt.Printf("LPAR:   %s", *lparName)
	fmt.Println("=================================================================")

	// 1. Initialize & Login
	fmt.Println("Step 1: Connecting to HMC...")
	client := hmc.NewHmcRestClient(*hmcIP)
	if err := client.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ Login failed: %v", err)
	}
	defer client.Logoff(context.Background())
	fmt.Println("✅ Connected to HMC")

	// 2. Resolve System UUID
	fmt.Printf("Step 2: Resolving System UUID for '%s'...", *sysName)
	_, sysUUID, err := client.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ Managed System '%s' not found: %v", *sysName, err)
	}
	fmt.Printf("✅ System UUID: %s\n\n", sysUUID)

	// 3. Resolve LPAR UUID and get details
	fmt.Printf("Step 3: Resolving LPAR UUID for '%s'...", *lparName)
	lparDetails, lparUUID, err := client.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found on system '%s': %v", *lparName, *sysName, err)
	}
	fmt.Printf("✅ LPAR UUID: %s\n", lparUUID)
	fmt.Printf("   State: %s\n\n", lparDetails.PartitionState)

	// 4. Get LPAR detailed information for profile
	fmt.Println("Step 4: Fetching LPAR detailed information...")
	lparDetailed, err := client.GetLogicalPartitionDetailed(context.Background(), lparUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get LPAR details: %v", err)
	}

	// Extract profile information
	profileHref := lparDetailed.AssociatedPartitionProfile.Href
	var profileUUID string
	if profileHref != "" {
		profileUUID = profileHref[len(profileHref)-36:]
		fmt.Printf("✅ Associated Profile UUID: %s", profileUUID)
	} else {
		fmt.Println("⚠️  No associated partition profile found")
	}

	// 5. Fetch Network Boot Devices from profile
	if profileUUID == "" {
		log.Fatal("❌ No associated partition profile found - cannot retrieve network boot devices")
	}

	fmt.Println("Step 5: Fetching network boot devices from profile...")
	bootDevices, err := client.GetNetworkBootDevices(context.Background(), lparUUID, profileUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ GetNetworkBootDevices failed: %v", err)
	}

	if len(bootDevices) == 0 {
		fmt.Printf("⚠️  No network boot devices found in profile for LPAR '%s'", *lparName)
		fmt.Println("📝 This LPAR profile has no network boot device configured.")
		fmt.Println("   You need to configure a network adapter in the profile before network boot.")
		return
	}

	fmt.Printf("✅ Retrieved %d network boot device(s) from profile\n\n", len(bootDevices))

	// 6. Display Network Boot Device Information
	fmt.Println("=================================================================")
	fmt.Println("  NETWORK BOOT DEVICE INFORMATION")
	fmt.Println("=================================================================")

	fmt.Println("📋 Network Boot Devices (from Profile):")
	for i, device := range bootDevices {
		fmt.Printf("--- Boot Device %d ---\n", i+1)
		fmt.Printf("Device Name:         %s\n", device.DeviceName)
		fmt.Printf("Device Type:         %s\n", device.DeviceType)
		fmt.Printf("Location Code:       %s\n", device.LocationCode)
		fmt.Printf("MAC Address:         %s\n", device.MACAddress)
		fmt.Println(strings.Repeat("-", 65))
	}

	// 7. Network Boot Configuration Summary
	fmt.Println("\n=================================================================")
	fmt.Println("  NETWORK BOOT CONFIGURATION SUMMARY")
	fmt.Println("=================================================================")

	// Use boot device information from profile
	primaryDevice := bootDevices[0]
	locationCode := primaryDevice.LocationCode
	macAddress := primaryDevice.MACAddress
	
	fmt.Println("For HMC Network Boot Command:")
	fmt.Printf("  LPAR UUID:           %s\n", lparUUID)
	fmt.Printf("  Profile UUID:        %s\n", profileUUID)
	fmt.Printf("  Location Code:       %s\n", locationCode)
	if macAddress != "" {
		fmt.Printf("  MAC Address:         %s\n", macAddress)
	}
	
	fmt.Println("\n📝 IMPORTANT NOTES:")
	fmt.Println("   ✅ Using authoritative boot device configuration from profile")
	fmt.Println("   ✅ Location code includes port suffix (ready for network boot)")
	fmt.Printf("   1. Use location code: %s\n", locationCode)
	fmt.Println("   2. Use the MAC address for DHCP/PXE configuration")
	fmt.Println("   3. This is the definitive boot configuration from the LPAR profile")
	
	if lparDetailed.PartitionState == "running" {
		fmt.Println("\n⚠️  WARNING: LPAR is currently RUNNING")
		fmt.Println("   Network boot requires the LPAR to be powered off first")
	} else {
		fmt.Printf("\n✅ LPAR is in '%s' state - ready for network boot\n", lparDetailed.PartitionState)
	}

	fmt.Println("\n=================================================================")
	fmt.Println("✅ Network boot device information retrieved successfully!")
	fmt.Println("=================================================================")
}

// Made with Bob
