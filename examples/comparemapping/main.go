package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS")
	vgName := flag.String("vg-name", "auto_vg01", "Target Volume Group")
	lparName := flag.String("lpar-name", "ocp-sno-lpar", "Target LPAR Name")
	
	// A temporary disk we will create just to watch the HMC map it
	diskName := flag.String("disk-name", "test_magic_lv", "Temporary disk to create and map")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *sysName == "" || *lparName == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, and lpar-name are required.")
	}

	fmt.Println("=========================================================================")
	fmt.Println(" 🔍 HMC Mapping Magic: Live Data Comparison")
	fmt.Println("=========================================================================")

	// =========================================================================
	// 1. AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("❌ HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	sysUUID, _, err := restClient.GetManagedSystemByName(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	viosUUID, err := hmc.GetViosID(restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found.", *viosName)
	}

	_, lparUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}

	// =========================================================================
	// 2. CREATE A TEMPORARY DISK
	// =========================================================================
	fmt.Printf("\n[Step 1] Creating a temporary 1GB disk '%s' on VIOS...\n", *diskName)
	err = restClient.CreateVirtualDisk(*sysName, viosUUID, *viosName, *vgName, *diskName, 1024, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to create Virtual Disk: %v", err)
	}

	// =========================================================================
	// 3. EXECUTE MAPPING (THE INJECTION)
	// =========================================================================
	fmt.Printf("\n[Step 2] Sending minimal Desired State XML to HMC...\n")
	fmt.Printf("         (Injecting ONLY <DiskName>%s</DiskName> into the DOM)\n", *diskName)
	
	_, err = restClient.CreateVirtualDiskMaps(sysUUID, viosUUID, lparUUID, []string{*diskName}, *verbose)
	if err != nil {
		log.Fatalf("❌ Storage Mapping Failed: %v", err)
	}
	fmt.Println("         ✅ HMC accepted the payload and triggered background configuration.")

	// =========================================================================
	// 4. WAIT FOR CACHE TO SETTLE
	// =========================================================================
	fmt.Println("\n[Step 3] Waiting 10 seconds for the HMC to generate the adapters and sync its XML cache...")
	time.Sleep(10 * time.Second)

	// =========================================================================
	// 5. FETCH LIVE DATA (THE AFTERMATH)
	// =========================================================================
	fmt.Println("\n[Step 4] Fetching LIVE VIOS XML Data...")
	
	mappings, err := restClient.GetViosSCSIMappings(viosUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get VIOS mappings: %v", err)
	}

	// Find our specific disk in the live data
	targetLparLower := strings.ToLower(lparUUID)
	var liveMapping hmc.VirtualSCSIMapping

	for _, mapping := range mappings {
		if strings.HasSuffix(strings.ToLower(mapping.AssociatedLogicalPartition.Href), targetLparLower) {
			if mapping.Storage.VirtualDisk.DiskName == *diskName {
				liveMapping = mapping
				break
			}
		}
	}

	if liveMapping.Storage.VirtualDisk.DiskName == "" {
		log.Fatalf("❌ Could not find the disk in the live XML. Cache might be severely lagging.")
	}

	// =========================================================================
	// 6. DISPLAY THE "HMC MAGIC"
	// =========================================================================
	fmt.Println("\n=========================================================================")
	fmt.Println(" 🎉 WHAT THE HMC AUTOGENERATED FOR YOU:")
	fmt.Println("=========================================================================")
	
	// Marshal the exact struct block to pretty JSON so you can see all the populated fields
	prettyJSON, _ := json.MarshalIndent(liveMapping, "", "    ")
	fmt.Println(string(prettyJSON))

	fmt.Println("\n=========================================================================")
	fmt.Println(" 💡 Notice how ClientAdapter and ServerAdapter are fully populated!")
	fmt.Printf("    - VIOS vhost adapter created: %s\n", liveMapping.ServerAdapter.AdapterName)
	fmt.Printf("    - VIOS slot assigned: %d\n", liveMapping.ServerAdapter.VirtualSlotNumber)
	fmt.Printf("    - LPAR client slot assigned: %d\n", liveMapping.ClientAdapter.VirtualSlotNumber)
	fmt.Println("=========================================================================")

	// =========================================================================
	// 7. CLEANUP
	// =========================================================================
	fmt.Printf("\n[Step 5] Cleaning up (Unmapping and deleting '%s')...\n", *diskName)
	
	_, err = restClient.DeleteVirtualDiskMaps(sysUUID, viosUUID, lparUUID, []string{*diskName}, *verbose)
	if err != nil {
		fmt.Printf("⚠️ Failed to unmap disk: %v\n", err)
	} else {
		err = restClient.DeleteVirtualDisk(*sysName, *viosName, *diskName, *verbose)
		if err != nil {
			fmt.Printf("⚠️ Failed to delete disk: %v\n", err)
		} else {
			fmt.Println("✅ Environment restored to clean state.")
		}
	}
		// =========================================================================
	// 5. FETCH LIVE DATA (THE AFTERMATH)
	// =========================================================================
	fmt.Println("\n[Step 4] Fetching LIVE VIOS XML Data...")
	
	mappings, err = restClient.GetViosSCSIMappings(viosUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get VIOS mappings: %v", err)
	}

	// Find our specific disk in the live data
	targetLparLower = strings.ToLower(lparUUID)
	var liveMappings hmc.VirtualSCSIMapping

	for _, mapping := range mappings {
		if strings.HasSuffix(strings.ToLower(mapping.AssociatedLogicalPartition.Href), targetLparLower) {
			if mapping.Storage.VirtualDisk.DiskName == *diskName {
				liveMappings = mapping
				break
			}
		}
	}

	if liveMapping.Storage.VirtualDisk.DiskName == "" {
		log.Fatalf("❌ Could not find the disk in the live XML. Cache might be severely lagging.")
	}

	// =========================================================================
	// 6. DISPLAY THE "HMC MAGIC"
	// =========================================================================
	fmt.Println("\n=========================================================================")
	fmt.Println(" 🎉 WHAT THE HMC AUTOGENERATED FOR YOU:")
	fmt.Println("=========================================================================")
	
	// Marshal the exact struct block to pretty JSON so you can see all the populated fields
	prettyJSON, _ = json.MarshalIndent(liveMappings, "", "    ")
	fmt.Println(string(prettyJSON))

	fmt.Println("\n=========================================================================")
	fmt.Println(" 💡 Notice how ClientAdapter and ServerAdapter are fully populated!")
	fmt.Printf("    - VIOS vhost adapter created: %s\n", liveMapping.ServerAdapter.AdapterName)
	fmt.Printf("    - VIOS slot assigned: %d\n", liveMapping.ServerAdapter.VirtualSlotNumber)
	fmt.Printf("    - LPAR client slot assigned: %d\n", liveMapping.ClientAdapter.VirtualSlotNumber)
	fmt.Println("=========================================================================")

}