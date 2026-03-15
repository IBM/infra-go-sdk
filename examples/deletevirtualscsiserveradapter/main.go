package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	hmc "github.com/sudeeshjohn/PowerHMC"
)

func main() {
	// ... [Existing Flag and Auth Code] ...
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	lparName := flag.String("lpar-name", "test-test-test", "LPAR Name to delete")
	verbose := flag.Bool("verbose", true, "Enable verbose output")
	flag.Parse()

	restClient := hmc.NewHmcRestClient(*hmcIP)
	restClient.Login(*username, *password, *verbose)
	defer restClient.Logoff()

	sysUUID, _, _ := restClient.GetManagedSystemByName(*sysName, *verbose)
	lpars, _ := restClient.GetLogicalPartitionsQuickAll(sysUUID, *verbose)
	
	var targetLparUUID string
	for _, l := range lpars {
		if l.PartitionName == *lparName {
			targetLparUUID = l.UUID
			break
		}
	}
	targetLparLower := strings.ToLower(targetLparUUID)

	// =========================================================================
	// DYNAMIC DISCOVERY & 5-STEP CLEANUP
	// =========================================================================
	vioses, _ := restClient.GetVirtualIOServersQuick(sysUUID, *verbose)

	for _, v := range vioses {
		// 1. Get ALL Server Adapters for this VIOS to build a Slot -> UUID map
		// This is the key to fixing the empty UUID issue.
		adapterList, err := restClient.GetVirtualSCSIServerAdapters(v.UUID, *verbose)
		if err != nil {
			continue
		}
		slotToUUID := make(map[string]string)
		for _, a := range adapterList {
			slotToUUID[a.VirtualSlotNumber] = a.UUID
		}

		// 2. Get the Mappings to find which ones belong to our LPAR
		mappings, err := restClient.GetViosSCSIMappings(v.UUID, *verbose)
		if err != nil {
			continue
		}

		for _, mapping := range mappings {
			assocLpar := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
			if assocLpar == nil { continue }
			
			href := strings.ToLower(assocLpar.SelectAttrValue("href", ""))
			if !strings.HasSuffix(href, targetLparLower) {
				continue
			}

			// --- EXTRACT DATA FROM MAPPING ---
			
			// Get Slot Number
			slotNum := ""
			serverAdapterElem := mapping.FindElement(".//*[local-name()='ServerAdapter']")
			if slotElem := serverAdapterElem.FindElement(".//*[local-name()='VirtualSlotNumber']"); slotElem != nil {
				slotNum = slotElem.Text()
			}

			// Cross-reference Slot Number to get the real UUID
			serverAdapterUUID := slotToUUID[slotNum]

			// Get VTD Name
			vtdName := ""
			if targetElem := mapping.FindElement(".//*[local-name()='TargetDevice']//*[local-name()='TargetName']"); targetElem != nil {
				vtdName = targetElem.Text()
			}

			// Get Volume Name
			backDev := mapping.FindElement(".//*[local-name()='ServerAdapter']/*[local-name()='BackingDeviceName']")
			volumeFound := "Unknown"
			if backDev != nil { volumeFound = backDev.Text() }

			fmt.Printf("\n🔎 Found Mapping: Volume=%s, VTD=%s, Slot=%s, AdapterUUID=%s\n", 
				volumeFound, vtdName, slotNum, serverAdapterUUID)

			// =========================================================================
			// THE 5-STEP SEQUENCE
			// =========================================================================

			// STEP 1: Delete Client Adapter (REST)
			_, _, err = restClient.RemoveVolumeLPARMapping(v.UUID, targetLparUUID, volumeFound, *verbose)
			if err != nil {
				fmt.Printf("   ⚠️ Step 1 Failed: %v\n", err)
			} else {
				fmt.Println("   ✅ Step 1: Client mapping removed.")
			}

			// STEP 2: Wait 10s
			fmt.Println("   ⏳ Step 2: Waiting 10s for VIOS cleanup...")
			time.Sleep(10 * time.Second)

			// STEP 3: Remove Backing Device (CLI)
			if vtdName != "" {
				fmt.Printf("   🚀 Step 3: Removing VTD %s via CLI...\n", vtdName)
				rmvdevCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmvdev -vtd %s"`, *sysName, v.PartitionName, vtdName)
				restClient.RunVIOSCommand(rmvdevCmd, *verbose)
			}

			// STEP 4: Wait 5s
			fmt.Println("   ⏳ Step 4: Waiting 5s for ODM sync...")
			time.Sleep(5 * time.Second)

			// STEP 5: Delete Server Adapter (REST)
			if serverAdapterUUID != "" {
				fmt.Printf("   🗑️ Step 5: Deleting Server Adapter UUID %s...\n", serverAdapterUUID)
				err := restClient.DeleteVirtualSCSIServerAdapter(v.UUID, serverAdapterUUID, *verbose)
				if err != nil {
					fmt.Printf("   ❌ Step 5 Failed: %v\n", err)
				} else {
					fmt.Println("   ✅ Step 5: Server adapter successfully deleted.")
				}
			} else {
				fmt.Println("   ⚠️ Step 5 Skipped: Could not resolve Adapter UUID for slot " + slotNum)
			}
		}
	}
	fmt.Println("\n🎉 Cleanup complete.")
}