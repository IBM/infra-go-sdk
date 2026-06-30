package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

func main() {
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	viosName := flag.String("vios-name", "", "Target VIOS Name")
	// Changed to default to empty string. If empty, we auto-detect it.
	profileName := flag.String("profile-name", "", "VIOS Profile Name (leave blank to auto-detect)")
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()

	// Validation
	if *hmcIP == "" || *username == "" || *password == "" || *sysName == "" || *viosName == "" {
		log.Fatal("❌ Error: hmc-ip, hmc-user, hmc-pass, system-name, and vios-name are required flags")
	}

	fmt.Println("=================================================================")
	fmt.Println("  VIOS Network Boot Device Utility (Immediate Mode)")
	fmt.Println("=================================================================")
	fmt.Printf("HMC:          %s\n", *hmcIP)
	fmt.Printf("System:       %s\n", *sysName)
	fmt.Printf("VIOS:         %s\n", *viosName)
	fmt.Printf("Logged User:  %s\n", *username)
	fmt.Println("=================================================================")

	// 1. Initialize & Login
	fmt.Println("\nStep 1: Connecting to HMC...")
	client := exutil.NewClient(*hmcIP, *debug, *debugFull)
	if err := client.Login(context.Background(), *username, *password); err != nil {
		log.Fatalf("❌ Login failed: %v", err)
	}
	defer client.Logoff(context.Background())
	fmt.Println("✅ Connected to HMC")

	// 2. Resolve System UUID
	fmt.Printf("Step 2: Resolving System UUID for '%s'...\n", *sysName)
	_, sysUUID, err := client.GetManagedSystemByNameQuick(context.Background(), *sysName)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ Managed System '%s' not found: %v", *sysName, err)
	}
	fmt.Printf("✅ System UUID: %s\n", sysUUID)

	// 3. Resolve VIOS UUID & Auto-Detect Profile Name
	fmt.Printf("Step 3: Resolving VIOS target '%s' and Profile...\n", *viosName)
	
	// Fetch the Quick list so we can dynamically grab the LastActivatedProfile
	viosList, err := client.GetVirtualIOServersQuick(context.Background(), sysUUID)
	if err != nil {
		log.Fatalf("❌ Failed to fetch VIOS list from system: %v", err)
	}

	var viosUUID string
	actualProfileName := *profileName

	for _, vios := range viosList {
		if strings.EqualFold(vios.PartitionName, *viosName) {
			viosUUID = vios.UUID
			
			// Auto-detect the profile if the user didn't explicitly override it via flag
			if actualProfileName == "" {
				actualProfileName = vios.LastActivatedProfile
			}
			break
		}
	}

	if viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found on system '%s'", *viosName, *sysName)
	}

	// Safety fallback in case the VIOS has never been activated and the field is empty
	if actualProfileName == "" {
		fmt.Println("⚠️  LastActivatedProfile was blank; falling back to 'default_profile'")
		actualProfileName = "default_profile"
	}

	fmt.Printf("✅ VIOS UUID: %s\n", viosUUID)
	fmt.Printf("✅ Target Profile: %s\n", actualProfileName)

	// 4. Fetch Network Boot Devices (Immediate Mode)
	fmt.Println("\nStep 4: Firing GetNetworkBootDevices Job...")
	bootDevices, err := client.GetNetworkBootDevicesForViosImmediate(
		context.Background(),
		viosUUID,
		*sysName,
		*viosName,
		actualProfileName, // <--- Passing the dynamically resolved profile
		*username,
	)

	if err != nil {
		log.Fatalf("❌ GetNetworkBootDevices failed: %v", err)
	}

	if len(bootDevices) == 0 {
		fmt.Printf("⚠️  No network boot devices found in profile '%s' for VIOS '%s'\n", actualProfileName, *viosName)
		return
	}

	fmt.Printf("✅ Retrieved %d network boot device(s) from profile\n", len(bootDevices))

	// 5. Display Network Boot Device Information
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
