package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	// Required to parse the pristine XML for idempotency check
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

	lparName := flag.String("lpar-name", "Go_LPAR_01", "Target LPAR Name")
	fcPortsRaw := flag.String("fc-ports", "", "Comma-separated list of physical FC ports (e.g., 'fcs0,fcs1') or WWPNs for deletion. Leave empty to map ALL discovered ports.")

	// Flags for Profile Saving and Deletion
	lparProfile := flag.String("lpar-profile", "default_profile", "Name of the LPAR profile to overwrite")
	forceSave := flag.Bool("force-save", true, "Force overwrite of the target profile")
	deleteMode := flag.Bool("delete", false, "Set to true to DELETE the NPIV mappings instead of creating them")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	flag.Parse()

	if *password == "" || *sysName == "" || *viosName == "" || *lparName == "" {
		log.Fatal("Error: hmc-pass, system-name, vios-name, and lpar-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("❌ HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
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
	// 1. DYNAMIC FIBRE CHANNEL PORT DISCOVERY & VALIDATION
	// =========================================================================
	fmt.Printf("\n[Validate] Discovering physical FC ports on VIOS '%s'...\n", *viosName)
	
	// ✨ USING THE NEW RESILIENT SDK FUNCTION ✨
	fcPorts, err := restClient.GetPhysicalFibreChannelPorts(viosUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch physical FC ports: %v", err)
	}

	var availablePorts []string
	for _, p := range fcPorts {
		if p.PortName != "" {
			availablePorts = append(availablePorts, p.PortName)
		}
	}

	if len(availablePorts) == 0 {
		log.Fatalf("❌ No Physical Fibre Channel ports found on this VIOS. NPIV mapping cannot proceed.")
	}

	var selectedPorts []string

	if *deleteMode && *fcPortsRaw != "" {
		// --- DELETE MODE: SMART TARGET BYPASS ---
		// User might pass WWPNs, Slot IDs, or Port Names. We skip physical validation
		// and pass the raw strings straight to the SDK's Smart Deleter.
		rawPorts := strings.Split(*fcPortsRaw, ",")
		seenPorts := make(map[string]bool)
		for _, rawPort := range rawPorts {
			cleanPort := strings.ToLower(strings.TrimSpace(rawPort))
			if cleanPort != "" && !seenPorts[cleanPort] {
				seenPorts[cleanPort] = true
				selectedPorts = append(selectedPorts, cleanPort)
			}
		}
		fmt.Printf("\n✅ [Smart Target Bypass] Accepted %d target(s) for deletion evaluation: %v\n", len(selectedPorts), selectedPorts)

	} else {
		// --- CREATE MODE: STRICT PHYSICAL PORT VALIDATION ---
		var missingPorts []string
		if *fcPortsRaw == "" {
			selectedPorts = availablePorts
			fmt.Printf("   -> Auto-selected ALL discovered Physical FC Ports: %v\n", selectedPorts)
		} else {
			rawPorts := strings.Split(*fcPortsRaw, ",")
			seenPorts := make(map[string]bool)

			for _, rawPort := range rawPorts {
				cleanPort := strings.ToLower(strings.TrimSpace(rawPort))
				if cleanPort == "" || seenPorts[cleanPort] {
					continue
				}
				seenPorts[cleanPort] = true

				found := false
				for _, ap := range availablePorts {
					if strings.EqualFold(ap, cleanPort) {
						found = true
						break
					}
				}

				if !found {
					missingPorts = append(missingPorts, cleanPort)
				} else {
					selectedPorts = append(selectedPorts, cleanPort)
				}
			}

			if len(missingPorts) > 0 {
				fmt.Printf("\n⚠️  Warning: The following Physical FC Ports do NOT exist on VIOS '%s' and will be SKIPPED:\n", *viosName)
				for _, p := range missingPorts {
					fmt.Printf("   - %s\n", p)
				}
			}

			if len(selectedPorts) == 0 {
				log.Fatalf("\n❌ Cannot proceed: None of the requested FC ports exist on the VIOS. Available ports: %v", availablePorts)
			}

			fmt.Printf("\n✅ %d valid, unique FC Port(s) ready for processing: %v\n", len(selectedPorts), selectedPorts)
		}
	}

	// =========================================================================
	// DIRECTORY & FILENAME PREPARATION
	// =========================================================================
	outDir := "outs"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("❌ Failed to create output directory: %v", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	safePortTag := strings.Join(selectedPorts, "-")
	
	beforeFile := fmt.Sprintf("%s/vios_before_vfc_%s_%s.xml", outDir, safePortTag, timestamp)
	afterFile := fmt.Sprintf("%s/vios_after_vfc_%s_%s.xml", outDir, safePortTag, timestamp)

	// =========================================================================
	// 2. DUMP "BEFORE" XML & CHECK CURRENT MAPPINGS
	// =========================================================================
	fmt.Printf("\n[Diff Tool] Fetching 'BEFORE' XML state (ViosFCMapping)...\n")
	
	beforeXML, err := restClient.GetRawViosXML(viosUUID, "ViosFCMapping", *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch before XML: %v", err)
	}

	err = os.WriteFile(beforeFile, []byte(beforeXML), 0644)
	if err != nil {
		log.Fatalf("❌ Failed to write before file: %v", err)
	}
	fmt.Printf("   -> Saved '%s'\n", beforeFile)

	// =========================================================================
	// 3. EXECUTE THE OPERATION (CREATE OR DELETE)
	// =========================================================================
	var operationStatus string

	if *deleteMode {
		// Because the SDK uses Smart Targets, we bypass local alreadyMappedPorts checks 
		// and hand the targets directly to the SDK!
		fmt.Printf("\n⚠️  Attempting to DELETE mapping targets %v from LPAR '%s'...\n", selectedPorts, *lparName)
		
		operationStatus, err = restClient.DeleteVirtualFibreChannelMaps(sysUUID, viosUUID, lparUUID, selectedPorts, *verbose)
		if err != nil {
			log.Fatalf("❌ vFC Deletion Failed: %v", err)
		}

		if operationStatus == "NOT_FOUND" {
			fmt.Printf("\n✅ The specified targets were not found on this LPAR. No action needed.\n")
		} else {
			fmt.Printf("\n🗑️  Deletion operation completed! Status: %s\n", operationStatus)
		}

	} else {
		// --- ADDITIVE CREATE LOGIC ---
		// We no longer skip already mapped ports. If the user passes 'fcs0', we map it.
		// If they run the script again, we map it again, generating new WWPNs!
		fmt.Printf("\n⚠️  Attempting to MAP %d vFC port(s) to LPAR '%s'...\n", len(selectedPorts), *lparName)
		operationStatus, err = restClient.CreateVirtualFibreChannelMaps(sysUUID, viosUUID, lparUUID, selectedPorts, *verbose)
		if err != nil {
			log.Fatalf("❌ vFC Mapping Failed: %v", err)
		}
		
		if operationStatus == "ALREADY_MAPPED" {
			fmt.Printf("\n✅ No action needed.\n")
		} else {
			fmt.Printf("\n💾 Mapping operation completed! Status: %s\n", operationStatus)
		}
	}

	// =========================================================================
	// 4. DUMP "AFTER" XML
	// =========================================================================
	fmt.Printf("\n[Diff Tool] Fetching 'AFTER' XML state (ViosFCMapping)...\n")
	afterXML, err := restClient.GetRawViosXML(viosUUID, "ViosFCMapping", *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch after XML: %v", err)
	}

	err = os.WriteFile(afterFile, []byte(afterXML), 0644)
	if err != nil {
		log.Fatalf("❌ Failed to write after file: %v", err)
	}
	fmt.Printf("   -> Saved '%s'\n", afterFile)

	// =========================================================================
	// 5. SAVE LPAR PROFILE (Only if topology changes were made)
	// =========================================================================
	if operationStatus == "SUCCESS" || operationStatus == "SUCCESS_WITH_RMC_WARNING" {
		fmt.Printf("\n[Profile] Saving running configuration to LPAR profile '%s'...\n", *lparProfile)
		saveErr := restClient.SaveCurrentLparConfig(lparUUID, *lparProfile, *forceSave, *verbose)
		if saveErr != nil {
			log.Printf("⚠️ Warning: vFC topology modified dynamically, but failed to save LPAR profile: %v\n", saveErr)
		} else {
			fmt.Println("✅ Success: LPAR profile saved. The Client Fibre Channel adapters will persist across reboots.")
		}
	} else {
		fmt.Printf("\n[Profile] No architectural changes were made to the LPAR. Profile save skipped.\n")
	}

	// =========================================================================
	// 6. AUDIT & DISPLAY NPIV TOPOLOGY
	// =========================================================================
	// We only show this if we are not deleting
	if !*deleteMode {
		fmt.Printf("\n[Audit] Fetching updated NPIV Mapping Details...\n")
		mappings, auditErr := restClient.GetVirtualFibreChannelMaps(viosUUID, lparUUID, *verbose)
		if auditErr != nil {
			fmt.Printf("⚠️  Failed to retrieve mapping details: %v\n", auditErr)
		} else if len(mappings) == 0 {
			fmt.Println("⚠️  No active FC mappings found for this LPAR.")
		} else {
			fmt.Printf("\n🔗 Current NPIV Topology for LPAR '%s':\n", *lparName)
			for _, m := range mappings {
				// Fallbacks in case the XML structure varies slightly
				wwpns := m.ClientAdapter.WWPNs
				if wwpns == "" {
					wwpns = "Pending (LPAR Offline)"
				}
				fmt.Printf("   - Physical Port: %-6s | Server: %-8s (Slot %v) -> Client Slot: %v | WWPNs: [%s]\n",
					m.Port.PortName, m.ServerAdapter.AdapterName, m.ServerAdapter.VirtualSlotNumber, m.ClientAdapter.VirtualSlotNumber, wwpns)
			}
		}
	}

	fmt.Println("\n=========================================================================")
	fmt.Printf(" 🎉 NPIV TEST COMPLETE! You can now diff the XML files:\n")
	fmt.Printf("    Linux/Mac: diff %s %s\n", beforeFile, afterFile)
	fmt.Printf("    VS Code:   code --diff %s %s\n", beforeFile, afterFile)
	fmt.Println("=========================================================================")
}