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
	lparName := flag.String("lpar-name", "ocp-sno-lpar", "Target Client LPAR Name")
	lparProfile := flag.String("lpar-profile", "default_profile", "Target LPAR Profile Name")
	
	// Optical Media and Topology parameters
	mediaName := flag.String("media-name", "test.iso", "Name of the ISO file in the VIOS repository")
	viosSlot := flag.Int("vios-slot", 51, "Target Virtual Slot Number on the VIOS (e.g., 51)")
	clientSlot := flag.Int("client-slot", 51, "Target Virtual Slot Number on the Client LPAR (e.g., 51)")

	verbose := flag.Bool("verbose", true, "Enable verbose output")
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose

	if *password == "" || *sysName == "" || *viosName == "" || *lparName == "" {
		log.Fatal("❌ Error: Missing required arguments.")
	}

	fmt.Println("=========================================================================")
	fmt.Printf(" 💿 ZERO-TOUCH HYBRID OPTICAL MEDIA ORCHESTRATION\n")
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
	// 2. NEGOTIATE HARDWARE TOPOLOGY (REST API)
	// =========================================================================
	fmt.Printf("\n🔌 Step 1: Provisioning Hardware Topology...\n")
	
	fmt.Printf("   -> Building Server Adapter on VIOS (Slot %d) pointing to LPAR ID %d...\n", *viosSlot, lparID)
	_, err = restClient.CreateVirtualSCSIServerAdapter(viosUUID, lparID, *viosSlot, *clientSlot)
	if err != nil {
		// It's common for an adapter to already exist on idempotency runs
		fmt.Printf("   ⚠️ Server Adapter creation returned: %v\n", err)
	}

	fmt.Printf("   -> Building Client Adapter on LPAR pointing to VIOS ID %d (Slot %d)...\n", viosID, *viosSlot)
	_, err = restClient.CreateVirtualSCSIClientAdapter(lparUUID, viosID, *viosSlot)
	if err != nil {
		fmt.Printf("   ⚠️ Client Adapter creation returned: %v\n", err)
	}

	// =========================================================================
	// 3. DISCOVER NEW HARDWARE ON VIOS
	// =========================================================================
	fmt.Printf("\n🔄 Step 2: Running cfgdev to initialize new hardware...\n")
	restClient.CliRunner(context.Background(), fmt.Sprintf(`viosvrcmd -m %s -p %s -c "cfgdev"`, *sysName, *viosName))
	time.Sleep(3 * time.Second) // Give the VIOS kernel a moment to create the vhost device

	fmt.Printf("🔍 Scanning VIOS for the new vhost adapter...\n")
	// Look for the vhost assigned to our exact Virtual Slot (e.g., C51)
	lsmapCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "lsmap -all"`, *sysName, *viosName)
	lsmapOut, _ := restClient.CliRunner(context.Background(), lsmapCmd)
	
	var vhostName string
	targetSlotStr := fmt.Sprintf("-C%d", *viosSlot)
	
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
	// 4. INSTANT OPTICAL MAPPING (VIOS CLI)
	// =========================================================================
	fmt.Printf("\n🚀 Step 3: Mapping Optical Media '%s' to '%s'...\n", *mediaName, vhostName)

	// A. Create the Virtual Optical Target Device (FBO - File Backed Optical)
	fmt.Printf("   -> Creating Virtual Optical Drive on %s...\n", vhostName)
	mkvdevCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mkvdev -fbo -vadapter %s"`, *sysName, *viosName, vhostName)
	mkvdevOut, err := restClient.CliRunner(context.Background(), mkvdevCmd)
	
	var vtoptName string
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error() + mkvdevOut), "already") {
			fmt.Println("   [Skip] Virtual Optical Drive already exists.")
			// We need to parse lsmap again to find the existing vtopt name
			lsmapVhostCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "lsmap -vadapter %s"`, *sysName, *viosName, vhostName)
			vhostOut, _ := restClient.CliRunner(context.Background(), lsmapVhostCmd)
			for _, line := range strings.Split(vhostOut, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "VTD") && strings.Contains(line, "vtopt") {
					fields := strings.Fields(line)
					if len(fields) > 1 {
						vtoptName = fields[1]
					}
					break
				}
			}
		} else {
			log.Fatalf("❌ Failed to create virtual optical drive: %v\nOutput: %s", err, mkvdevOut)
		}
	} else {
		// Output usually looks like "vtopt0 Available"
		fields := strings.Fields(strings.TrimSpace(mkvdevOut))
		if len(fields) > 0 {
			vtoptName = fields[0]
			fmt.Printf("   ✅ Created Virtual Optical Drive: %s\n", vtoptName)
		}
	}

	if vtoptName == "" {
		log.Fatalf("❌ Failed to determine Virtual Optical Drive (vtopt) name.")
	}

	// B. Load the ISO into the Virtual Optical Drive
	fmt.Printf("   -> Loading '%s' into %s...\n", *mediaName, vtoptName)
	loadoptCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "loadopt -vtd %s -disk %s"`, *sysName, *viosName, vtoptName, *mediaName)
	loadoptOut, err := restClient.CliRunner(context.Background(), loadoptCmd)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error() + loadoptOut), "already loaded") {
			fmt.Println("   [Skip] Media is already loaded.")
		} else {
			log.Fatalf("❌ Failed to load ISO: %v\nOutput: %s", err, loadoptOut)
		}
	} else {
		fmt.Printf("   ✅ ISO successfully loaded into %s!\n", vtoptName)
	}

	// =========================================================================
	// 5. PERSISTENCE (SAVE PROFILES)
	// =========================================================================
	fmt.Printf("\n💾 Step 4: Saving Profiles for Reboot Safety...\n")
	
	fmt.Printf("   -> Saving LPAR Profile...\n")
	restClient.SaveCurrentLparConfig(context.Background(), lparUUID, *lparProfile, true)

	fmt.Println("\n=========================================================================")
	fmt.Printf(" 🎉 OPTICAL PROVISIONING COMPLETE!\n")
	fmt.Printf("    LPAR: %s is securely cabled to VIOS: %s\n", *lparName, *viosName)
	fmt.Printf("    Media: %s is loaded and ready for boot.\n", *mediaName)
	fmt.Println("=========================================================================")
}
