package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")

	sysName := flag.String("system-name", "", "Managed System Name")
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS Name")
	vgName := flag.String("vg-name", "auto_vg01", "Target Volume Group")
	lparName := flag.String("lpar-name", "ocp-sno-lpar", "Target Client LPAR Name")
	// ADD THIS LINE:
	lparProfile := flag.String("lpar-profile", "default_profile", "Target LPAR Profile Name")
	
	// Storage and Topology parameters
	diskName := flag.String("disk-name", "hybrid_disk_01", "Name of the new Virtual Disk (LV)")
	diskSize := flag.Int("disk-size", 1024, "Size of the Virtual Disk in Megabytes")
	viosSlot := flag.Int("vios-slot", 50, "Target Virtual Slot Number on the VIOS")
	clientSlot := flag.Int("client-slot", 50, "Target Virtual Slot Number on the Client LPAR")

	verbose := flag.Bool("verbose", false, "Enable verbose output")
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose

	if *password == "" || *sysName == "" || *viosName == "" || *lparName == "" {
		log.Fatal("❌ Error: Missing required arguments.")
	}

	fmt.Println("=========================================================================")
	fmt.Printf(" 🚀 ZERO-TOUCH HYBRID STORAGE ORCHESTRATION\n")
	fmt.Println("=========================================================================")

	// =========================================================================
	// 1. AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)
	if err := restClient.Login(context.Background(), *username, *password); err != nil {
		log.Fatalf("❌ HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	fmt.Printf("\n🔍 Resolving System, VIOS, and LPAR IDs...\n")
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	// Resolve LPAR
	lparDetails, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName)
	if err != nil || lparDetails == nil {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}
	lparID := lparDetails.PartitionID

	// Resolve VIOS
	viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID)
	if err != nil {
		log.Fatalf("❌ Failed to fetch VIOS instances: %v", err)
	}
	var viosUUID string
	var viosID int
	for _, v := range viosList {
		if strings.EqualFold(v.PartitionName, *viosName) {
			viosUUID = v.UUID
			viosID = v.PartitionID
			break
		}
	}
	if viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found.", *viosName)
	}
	
	fmt.Printf("✅ Resolution complete.\n   - LPAR UUID: %s (ID: %d)\n   - VIOS UUID: %s (ID: %d)\n", lparUUID, lparID, viosUUID, viosID)

	// =========================================================================
	// 2. CREATE THE VIRTUAL DISK (VIOS CLI)
	// =========================================================================
	fmt.Printf("\n💽 Step 1: Creating Logical Volume '%s' (%d MB)...\n", *diskName, *diskSize)
	err = restClient.CreateVirtualDisk(context.Background(), *sysName, viosUUID, *viosName, *vgName, *diskName, *diskSize)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			fmt.Printf("   [Skip] Disk '%s' already exists.\n", *diskName)
		} else {
			log.Fatalf("❌ Failed to create disk: %v", err)
		}
	}

	// =========================================================================
	// 3. NEGOTIATE HARDWARE TOPOLOGY (REST API)
	// =========================================================================
	fmt.Printf("\n🔌 Step 2: Provisioning Hardware Topology...\n")
	
	fmt.Printf("   -> Building Server Adapter on VIOS (Slot %d) pointing to LPAR ID %d...\n", *viosSlot, lparID)
	_, err = restClient.CreateVirtualSCSIServerAdapter(viosUUID, lparID, *viosSlot, *clientSlot)
	if err != nil {
		// It's common for an adapter to already exist on idempotency runs, so we warn instead of fatal
		fmt.Printf("   ⚠️ Server Adapter creation returned: %v\n", err)
	}

	fmt.Printf("   -> Building Client Adapter on LPAR pointing to VIOS ID %d (Slot %d)...\n", viosID, *viosSlot)
	_, err = restClient.CreateVirtualSCSIClientAdapter(lparUUID, viosID, *viosSlot)
	if err != nil {
		fmt.Printf("   ⚠️ Client Adapter creation returned: %v\n", err)
	}

	// =========================================================================
	// 4. DISCOVER NEW HARDWARE ON VIOS
	// =========================================================================
	fmt.Printf("\n🔄 Step 3: Running cfgdev to initialize new hardware...\n")
	restClient.CliRunner(context.Background(), fmt.Sprintf(`viosvrcmd -m %s -p %s -c "cfgdev"`, *sysName, *viosName))
	time.Sleep(3 * time.Second) // Give the VIOS kernel a moment to create the vhost device

	fmt.Printf("🔍 Scanning VIOS for the new vhost adapter...\n")
	// Look for the vhost assigned to our exact Virtual Slot (e.g., C50)
	lsmapCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "lsmap -all"`, *sysName, *viosName)
	lsmapOut, _ := restClient.CliRunner(context.Background(), lsmapCmd)
	
	var vhostName string
	targetSlotStr := fmt.Sprintf("-C%d", *viosSlot) // e.g., "-C50"
	
	for _, line := range strings.Split(lsmapOut, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "vhost") && strings.Contains(line, targetSlotStr) {
			fields := strings.Fields(line)
			vhostName = fields[0]
			break
		}
	}

	if vhostName == "" {
		log.Fatalf("❌ Failed to find the newly created vhost adapter at slot %d in lsmap output.", *viosSlot)
	}
	fmt.Printf("✅ VIOS assigned OS name: %s\n", vhostName)

	// =========================================================================
	// 5. INSTANT STORAGE MAPPING (VIOS CLI)
	// =========================================================================
	fmt.Printf("\n🚀 Step 4: Mapping Disk '%s' to '%s'...\n", *diskName, vhostName)
	mkvdevCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mkvdev -vdev %s -vadapter %s"`, *sysName, *viosName, *diskName, vhostName)
	output, err := restClient.CliRunner(context.Background(), mkvdevCmd)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error() + output), "already mapped") || strings.Contains(strings.ToLower(output), "available") {
			fmt.Println("   [Skip] Disk is already mapped.")
		} else {
			log.Fatalf("❌ mkvdev mapping failed: %v\nOutput: %s", err, output)
		}
	} else {
		fmt.Printf("✅ Disk mapped successfully! Output: %s\n", strings.TrimSpace(output))
	}

	// =========================================================================
	// 6. PERSISTENCE (SAVE PROFILES)
	// =========================================================================
	fmt.Printf("\n💾 Step 5: Saving Profiles for Reboot Safety...\n")
	
	fmt.Printf("   -> Saving LPAR Profile...\n")
	// Use the pointer to the flag we defined at the top of the script!
	restClient.SaveCurrentLparConfig(context.Background(), lparUUID, *lparProfile, true)	
	
	//fmt.Printf("   -> Saving VIOS Profile...\n")
	// Replace "default_profile" with your actual VIOS profile name if different
	//restClient.SaveCurrentViosConfig(viosUUID, "default_profile", true) 

	fmt.Println("\n=========================================================================")
	fmt.Printf(" 🎉 END-TO-END PROVISIONING COMPLETE!\n")
	fmt.Printf("    LPAR: %s is securely cabled to VIOS: %s\n", *lparName, *viosName)
	fmt.Printf("    Disk: %s is available and permanently mapped.\n", *diskName)
	fmt.Println("=========================================================================")
}
