package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	hmc "github.com/sudeeshjohn/PowerHMC" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP")
	user := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC User")
	pass := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC Password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Target Managed System")
	lparName := flag.String("lpar-name", "Go_LPAR_03", "Name for the new LPAR")
	
	// Networking Flags
	vswitchName := flag.String("vswitch-name", "VNET0", "Name of the Virtual Switch")
	vlanID := flag.Int("vlan-id", 1, "VLAN ID for the Client Network Adapter")

	// Storage Flags
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS for storage mapping")
	diskName := flag.String("disk-name", "hdisk3", "Name of the physical volume on the VIOS")

	verbose := flag.Bool("verbose", false, "Verbose logs")
	flag.Parse()

	// =========================================================================
	// 1. AUTHENTICATION & SYSTEM RESOLUTION
	// =========================================================================
	client := hmc.NewHmcRestClient(*hmcIP)
	if err := client.Login(*user, *pass, *verbose); err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	defer client.Logoff()

	fmt.Printf("🔍 Finding system %s...\n", *sysName)
	systems, err := client.GetManagedSystemQuickAll(*verbose)
	if err != nil {
		log.Fatalf("Failed to fetch managed systems: %v", err)
	}
	
	var sysUUID string
	for _, s := range systems {
		if strings.EqualFold(s.SystemName, *sysName) {
			sysUUID = s.UUID
			break
		}
	}
	if sysUUID == "" {
		log.Fatalf("❌ System %s not found.", *sysName)
	}

	// =========================================================================
	// 2. CREATE LOGICAL PARTITION (LPAR)
	// =========================================================================
	// Define Creation Request (0.5 CPU / 4GB RAM)
	req := hmc.CreateLparRequest{
		Name:             *lparName,
		MinMem:           2048,
		DesiredMem:       4096,
		MaxMem:           8192,
		MinProcUnits:     0.1,
		DesiredProcUnits: 0.5,
		MaxProcUnits:     2.0,
		MinVcpus:         1,
		DesiredVcpus:     1,
		MaxVcpus:         4,
		SharingMode:      "uncapped",
	}

	fmt.Printf("\n🚀 Step 1: Provisioning base LPAR '%s'...\n", *lparName)
	lparUUID, err := client.CreateLogicalPartition(sysUUID, req, *verbose)
	if err != nil {
		log.Fatalf("❌ LPAR Creation failed: %v", err)
	}
	fmt.Printf("   ✅ LPAR Created! UUID: %s\n", lparUUID)

	// =========================================================================
	// 3. ATTACH VIRTUAL ETHERNET ADAPTER (NETWORK)
	// =========================================================================
	fmt.Printf("\n🌐 Step 2: Resolving Virtual Switch '%s'...\n", *vswitchName)
	switches, err := client.GetVirtualSwitchQuickAll(sysUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve Virtual Switches: %v", err)
	}

	var vswitchUUID string
	for _, s := range switches {
		if strings.EqualFold(s.SwitchName, *vswitchName) {
			vswitchUUID = s.UUID
			break
		}
	}
	if vswitchUUID == "" {
		log.Fatalf("❌ Virtual Switch '%s' not found on system '%s'.", *vswitchName, *sysName)
	}

	fmt.Printf("   🔌 Attaching VLAN %d (Switch: %s) to LPAR %s...\n", *vlanID, *vswitchName, *lparName)
	adapterUUID, err := client.CreateClientNetworkAdapter(sysUUID, lparUUID, vswitchUUID, *vlanID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to add network adapter: %v", err)
	}
	fmt.Printf("   ✅ Network Adapter Attached! UUID: %s\n", adapterUUID)

	// =========================================================================
	// 4. ATTACH PHYSICAL VOLUME (STORAGE)
	// =========================================================================
	fmt.Printf("\n💾 Step 3: Resolving VIOS '%s'...\n", *viosName)
	viosUUID, err := hmc.GetViosID(client, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found on system '%s'.", *viosName, *sysName)
	}

	fmt.Printf("   ⚠️  Mapping Physical Volume '%s' from VIOS '%s' to LPAR '%s'...\n", *diskName, *viosName, *lparName)
	mappingUUID, err := client.CreatePhysicalVolumeMap(sysUUID, viosUUID, lparUUID, *diskName, *verbose)
	if err != nil {
		log.Fatalf("❌ Storage Mapping Failed: %v", err)
	}
	
	// Because the LPAR is freshly created and not booted, we expect the RMC warning!
	if mappingUUID == "SUCCESS_WITH_RMC_WARNING" {
		fmt.Printf("   ✅ Disk mapped successfully! (Ignored expected RMC warning for offline LPAR)\n")
	} else {
		fmt.Printf("   ✅ Disk mapped successfully! Mapping Output: %s\n", mappingUUID)
	}

	// =========================================================================
	// 5. SUMMARY
	// =========================================================================
	fmt.Printf("\n🎉 SUCCESS: PowerVM Provisioning Workflow Complete!\n")
	fmt.Printf("   - LPAR Name : %s\n", *lparName)
	fmt.Printf("   - LPAR UUID : %s\n", lparUUID)
	fmt.Printf("   - Network   : Attached to %s (VLAN %d)\n", *vswitchName, *vlanID)
	fmt.Printf("   - Storage   : Mapped %s via %s\n", *diskName, *viosName)
	fmt.Println("=========================================================================")
}