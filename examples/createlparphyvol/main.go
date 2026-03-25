package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	hmc "github.com/sudeeshjohn/powerhmc-go"
	svc "github.com/sudeeshjohn/svc-go-sdk"
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	// HMC Config
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC Username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC Password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	lparName := flag.String("lpar-name", "Go_LPAR_99", "Name for the new LPAR")
	osType := flag.String("os-type", "AIX/Linux", "OS type (AIX/Linux, OS400, Virtual IO Server)")

	// Networking Config
	vswitchName := flag.String("vswitch-name", "VNET0", "Name of the Virtual Switch")
	vlanID := flag.Int("vlan-id", 1, "VLAN ID for the Client Network Adapter")

	// SVC Config
	svcIP := flag.String("svc-ip", "192.0.2.8", "SVC IP address")
	svcUser := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC Username")
	svcPass := flag.String("svc-pass", "REDACTED_HMC_PASS<==", "SVC Password")
	baseImageName := flag.String("base-image", "image-ibm-default-centos-10", "Base image name for FlashCopy")

	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	// Derive volume name from LPAR name
	volumeName := fmt.Sprintf("%s_boot_vol", strings.ReplaceAll(*lparName, " ", "_"))

	log.Println("=========================================================================")
	log.Println(" 🚀 Starting Modern PowerVM LPAR Creation & SAN Provisioning")
	log.Println("=========================================================================")

	// =========================================================================
	// 1. PARALLEL AUTHENTICATION (HMC + SVC)
	// =========================================================================
	log.Println("")
	log.Println("🔀 Starting Parallel Authentication: HMC || SVC...")
	
	var restClient *hmc.HmcRestClient
	var svcclient *svc.Client
	var wg sync.WaitGroup
	var hmcErr, svcErr  error
	
	wg.Add(2)
	
	// Authenticate to HMC in parallel
	go func() {
		defer wg.Done()
		log.Println("[Auth-HMC] Connecting to HMC...")
		restClient = hmc.NewHmcRestClient(*hmcIP)
		if err := restClient.Login(*username, *password, *verbose); err != nil {
			hmcErr = fmt.Errorf("HMC login failed: %v", err)
			return
		}
		log.Println("[Auth-HMC] ✅ HMC Authentication Successful")
	}()
	
	// Authenticate to SVC in parallel
	go func() {
		defer wg.Done()
		log.Println("[Auth-SVC] Connecting to SVC...")
		svcclient = svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()
		if err := svcclient.Authenticate(); err != nil {
			svcErr = fmt.Errorf("SVC authentication failed: %v", err)
			return
		}
		log.Println("[Auth-SVC] ✅ SVC Authentication Successful")
	}()
	
	wg.Wait()
	
	// Check for authentication errors
	if hmcErr != nil {
		log.Fatalf("❌ %v", hmcErr)
	}
	if svcErr != nil {
		log.Fatalf("❌ %v", svcErr)
	}
	
	defer restClient.Logoff()
	
	log.Println("✅ Both authentications completed successfully")

	// =========================================================================
	// 2. SYSTEM RESOLUTION & VALIDATION
	// =========================================================================
	sysUUID := resolveSystemUUID(restClient, *sysName, *verbose)
	ensureLparDoesNotExist(restClient, sysUUID, *lparName, *verbose)

	// =========================================================================
	// 3. PARALLEL: CREATE LPAR || DISCOVER VIOS || RESOLVE VSWITCH
	// =========================================================================
	log.Println("")
	log.Println("🔀 Starting 3-Way Parallel Operations: LPAR Creation || VIOS Discovery || vSwitch Resolution...")
	
	var lparDetails *hmc.LogicalPartitionDetailed
	var lparUUID string
	var viosWwpnMap map[string][]string
	var viosUuidMap map[string]string
	var vswitchUUID string
	var lparErr, viosErr, vswitchErr error
	
	wg.Add(3)
	
	// Thread 1: Create LPAR
	go func() {
		defer wg.Done()
		log.Printf("[Thread-LPAR] Creating LPAR '%s'...", *lparName)
		req := hmc.CreateLparRequest{
			Name:             "Go_LPAR_99",
			OsType:           *osType, // ❌ Do not use "/Linux" or "Linux"
			MinMem:           2048,
			DesiredMem:       4096,
			MaxMem:           8192,
			MinProcUnits:     0.1,
			DesiredProcUnits: 0.5,
			MaxProcUnits:     2.0,
			MinVcpus:         1,
			DesiredVcpus:     2,
			MaxVcpus:         4,
			SharingMode:      "uncapped",
		}

		var err error
		lparDetails, err = restClient.CreateLogicalPartition(sysUUID, req, *verbose)
		if err != nil {
			lparErr = fmt.Errorf("LPAR creation failed: %v", err)
			return
		}
		lparUUID = lparDetails.MetadataID
		log.Printf("[Thread-LPAR] ✅ LPAR Created! UUID: %s", lparUUID)
	}()
	
	// Thread 2: Discover VIOS WWPNs
	go func() {
		defer wg.Done()
		log.Println("[Thread-VIOS] Discovering VIOS WWPNs...")
		var err error
		viosWwpnMap, viosUuidMap, err = getViosWwpnMap(restClient, sysUUID, *verbose)
		if err != nil {
			viosErr = fmt.Errorf("VIOS discovery failed: %v", err)
			return
		}
		log.Println("[Thread-VIOS] ✅ VIOS Discovery Complete.")
	}()
	
	// Thread 3: Resolve Virtual Switch
	go func() {
		defer wg.Done()
		log.Printf("[Thread-vSwitch] Resolving Virtual Switch '%s'...", *vswitchName)
		switches, err := restClient.GetVirtualSwitchQuickAll(sysUUID, *verbose)
		if err != nil {
			vswitchErr = fmt.Errorf("failed to retrieve Virtual Switches: %v", err)
			return
		}

		for _, s := range switches {
			if strings.EqualFold(s.SwitchName, *vswitchName) {
				vswitchUUID = s.UUID
				break
			}
		}
		if vswitchUUID == "" {
			vswitchErr = fmt.Errorf("virtual Switch '%s' not found", *vswitchName)
			return
		}
		log.Printf("[Thread-vSwitch] ✅ Virtual Switch Resolved: %s", vswitchUUID)
	}()
	
	wg.Wait()
	
	// Check for errors
	if lparErr != nil {
		log.Fatalf("❌ LPAR Thread Failed: %v", lparErr)
	}
	if viosErr != nil {
		log.Fatalf("❌ VIOS Thread Failed: %v", viosErr)
	}
	if vswitchErr != nil {
		log.Fatalf("❌ vSwitch Thread Failed: %v", vswitchErr)
	}
	
	log.Println("✅ All 3 parallel operations completed successfully")

	// =========================================================================
	// 4. PARALLEL: ATTACH NETWORK || PROVISION SVC STORAGE
	// =========================================================================
	log.Println("🔀 Starting Parallel Operations: Network Attachment || SVC Storage Provisioning...")
	
	var targetVol *svc.Vdisk
	var selectedViosName string
	var networkErr2, storageErr error
	
	wg.Add(2)
	
	// Thread 1: Attach Network Adapter
	go func() {
		defer wg.Done()
		log.Printf("[Thread-Network] Attaching VLAN %d to LPAR...", *vlanID)
		_, err := restClient.CreateClientNetworkAdapter(sysUUID, lparUUID, vswitchUUID, *vlanID, *verbose)
		if err != nil {
			networkErr2 = fmt.Errorf("failed to add network adapter: %v", err)
			return
		}
		log.Printf("[Thread-Network] ✅ Network Adapter Attached.")
	}()
	
	// Thread 2: Provision SVC Storage
	go func() {
		defer wg.Done()
		log.Println("[Thread-SVC] Starting SVC storage provisioning...")
		var err error
		targetVol, selectedViosName, err = provisionSVCStorage(svcclient, *baseImageName, viosWwpnMap, volumeName, *verbose)
		if err != nil {
			storageErr = fmt.Errorf("failed to provision SVC storage: %v", err)
			return
		}
		log.Println("[Thread-SVC] ✅ SVC Storage Provisioned.")
	}()
	
	wg.Wait()
	
	// Check for errors
	if networkErr2 != nil {
		log.Fatalf("❌ Network Thread Failed: %v", networkErr2)
	}
	if storageErr != nil {
		log.Fatalf("❌ Storage Thread Failed: %v", storageErr)
	}
	
	log.Println("✅ Both parallel operations completed successfully")

	// =========================================================================
	// 5. DISCOVER NEW DISK ON VIOS & MAP IT TO LPAR
	// =========================================================================
	log.Println("")
	log.Printf("[HMC] Configuring Storage on VIOS '%s'...", selectedViosName)
	viosUUID := viosUuidMap[selectedViosName]

	log.Printf("[HMC] Running ConfigDevice (cfgdev) to scan for the new SVC LUN...")
	if err := restClient.ConfigDevice(viosUUID, "", *verbose); err != nil {
		log.Fatalf("[HMC] Failed to run cfgdev: %v", err)
	}

	log.Printf("[HMC] Locating new physical volume matching SVC UID: %s...", targetVol.VdiskUID)
	diskName, err := identifyFreeVolume(restClient, viosUUID, selectedViosName, targetVol.VdiskUID, *verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to identify free volume: %v", err)
	}
	log.Printf("[HMC] ✅ Matched SVC LUN to VIOS Disk: %s", diskName)

	log.Printf("[HMC] Attaching '%s' to LPAR '%s'...", diskName, *lparName)
	mappingUUID, err := restClient.CreatePhysicalVolumeMap(sysUUID, viosUUID, lparUUID, []string{diskName}, *verbose)
	if err != nil {
		log.Fatalf("[HMC] Storage Mapping Failed: %v", err)
	}
	
	if mappingUUID == "SUCCESS_WITH_RMC_WARNING" {
		log.Printf("[HMC] ✅ Disk mapped successfully! (Ignored expected RMC warning for offline LPAR)")
	} else {
		log.Printf("[HMC] ✅ Disk mapped successfully!")
	}

	// =========================================================================
	// 6. SAVE CONFIGURATION & POWER ON THE LPAR
	// =========================================================================
	// Use the default profile name from the LPAR details
	profileName := lparDetails.DefaultProfileName
	log.Println("")
	log.Printf("[HMC] Saving active configuration to profile '%s'...", profileName)
	err = restClient.SaveCurrentLparConfig(lparUUID, profileName, true, *verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to save LPAR configuration: %v", err)
	}
	log.Printf("[HMC] ✅ Configuration permanently saved to profile.")

	log.Println("")
	log.Printf("[HMC] Step 7: Powering on LPAR '%s'...", *lparName)
	
	// Extract profile UUID from the AssociatedPartitionProfile href (already available from CreateLogicalPartition)
	profileHref := lparDetails.AssociatedPartitionProfile.Href
	if profileHref == "" {
		log.Fatalf("[HMC] No associated partition profile found for LPAR")
	}
	
	// Extract UUID from href (last 36 characters)
	if len(profileHref) < 36 {
		log.Fatalf("[HMC] Invalid profile href format: %s", profileHref)
	}
	profileUUID := profileHref[len(profileHref)-36:]
	
	if *verbose {
		log.Printf("[HMC] Using default profile '%s' (UUID: %s)", lparDetails.DefaultProfileName, profileUUID)
	}

	_, err = restClient.PowerOnPartition(lparUUID, profileUUID, "normal", "", *osType, "", "", "", "", "", "", *verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to PowerOn Partition: %v", err)
	}

	log.Println("")
	log.Println("=========================================================================")
	log.Printf(" 🎉 SUCCESS: PowerVM Provisioning Workflow Complete!")
	log.Printf("    - LPAR Name : %s is BOOTING", *lparName)
	log.Printf("    - Network   : Attached to %s (VLAN %d)", *vswitchName, *vlanID)
	log.Printf("    - Storage   : Mapped SVC Vol '%s' (%s) via %s", targetVol.Name, diskName, selectedViosName)
	log.Println("=========================================================================")
}

// =========================================================================
// WORKFLOW HELPER FUNCTIONS
// =========================================================================

func resolveSystemUUID(restClient *hmc.HmcRestClient, systemName string, verbose bool) string {
	systems, err := restClient.GetManagedSystemQuickAll(verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to get managed systems: %v", err)
	}
	for _, system := range systems {
		if strings.EqualFold(system.SystemName, systemName) {
			if verbose {
				log.Printf("[HMC] Resolved Managed System UUID: %s", system.UUID)
			}
			return system.UUID
		}
	}
	log.Fatalf("[HMC] Managed system '%s' not found.", systemName)
	return ""
}

func ensureLparDoesNotExist(restClient *hmc.HmcRestClient, systemUUID, vmName string, verbose bool) {
	if verbose {
		log.Printf("[HMC] Verifying LPAR name '%s' is unique...", vmName)
	}
	_,existingUUID, err := restClient.GetLogicalPartitionByName(systemUUID, vmName, false)
	if err == nil && existingUUID != "" {
		log.Fatalf("[HMC] Error: LPAR with name '%s' already exists (UUID: %s)", vmName, existingUUID)
	}
}

func getViosWwpnMap(restClient *hmc.HmcRestClient, systemUUID string, verbose bool) (map[string][]string, map[string]string, error) {
	viosList, err := restClient.GetVirtualIOServers(systemUUID, verbose)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch VIOS details: %v", err)
	}

	viosWwpnMap := make(map[string][]string)
	viosUuidMap := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := range viosList {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			v := viosList[idx]
			var wwpns []string
			for _, port := range v.Storage.FibreChannelPorts {
				if port.WWPN != "" {
					wwpns = append(wwpns, strings.ToUpper(port.WWPN))
				}
			}
			if len(wwpns) > 0 {
				mu.Lock()
				viosWwpnMap[v.PartitionName] = wwpns
				viosUuidMap[v.PartitionName] = v.UUID
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if len(viosWwpnMap) == 0 {
		return nil, nil, fmt.Errorf("no Fibre Channel WWPNs found on any VIOS")
	}
	return viosWwpnMap, viosUuidMap, nil
}

func provisionSVCStorage(svcclient *svc.Client, baseImageName string, viosWwpnMap map[string][]string, volumeName string, verbose bool) (*svc.Vdisk, string, error) {

	var selectedViosName string
	var selectedHostName string
	var selectedWWPNs []string
	hostExists := false

	fabricLogins, err := svcclient.Lsfabric()
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch fabric logins: %v", err)
	}
	
	wwpnToHostMap := make(map[string]string)
	for _, login := range fabricLogins {
		wwpnToHostMap[strings.ToUpper(login.RemoteWWPN)] = login.HostName
	}
	
	for viosName, wwpns := range viosWwpnMap {
		for _, wwpn := range wwpns {
			if hostName, found := wwpnToHostMap[strings.ToUpper(wwpn)]; found {
				if verbose {
					log.Printf("[SVC] ✅ Match Found! VIOS '%s' is mapped to SVC Host '%s'", viosName, hostName)
				}
				selectedViosName = viosName
				selectedHostName = hostName
				selectedWWPNs = wwpns
				hostExists = true
				break
			}
		}
		if hostExists { break }
	}

	if !hostExists {
		if verbose { log.Println("[SVC] ⚠️ No matching SVC host found. Preparing to create a new host mapping...") }
		for viosName, wwpns := range viosWwpnMap {
			selectedViosName = viosName
			selectedHostName = viosName
			selectedWWPNs = wwpns
			break
		}
		newHost := svc.Host{
			Name:     selectedHostName,
			Fcwwpn:   selectedWWPNs,
			Type:     "generic",
			Protocol: "scsi",
		}
		if err := svcclient.Mkhost(newHost); err != nil && !strings.Contains(err.Error(), "already exists") {
			return nil, "", fmt.Errorf("mkhost error: %v", err)
		}
	}

	targetHost, err := svcclient.LshostByTarget(selectedHostName)
	if err != nil {
		return nil, "", fmt.Errorf("error finding host %s: %v", selectedHostName, err)
	}

	volume := svc.Volume{
		Name: volumeName,
		MdiskGrp: "0", Size: 120, Unit: "gb",
		RSize: "2%", Warning: "80%", AutoExpand: true, GrainSize: 256,
	}
	if err := svcclient.Mkvdisk(volume); err != nil {
		return nil, "", fmt.Errorf("mkvdisk error: %v", err)
	}
	
	sourceVol, err := svcclient.LsVdiskByName(baseImageName)
	if err != nil {
		return nil, "", fmt.Errorf("error finding source volume %s: %v", baseImageName, err)
	}
	targetVol, err := svcclient.LsVdiskByName(volume.Name)
	if err != nil {
		return nil, "", fmt.Errorf("error finding target volume %s: %v", volume.Name, err)
	}

	fcmapping := svc.FlashCopyMapping{
		Name: fmt.Sprintf("fcmap_%d", time.Now().Unix()), Source: sourceVol.ID, Target: targetVol.ID,
		CopyRate: 150, GrainSize: 256, Incremental: true, AutoDelete: true,
	}
	if err := svcclient.Mkfcmap(fcmapping); err != nil {
		return nil, "", fmt.Errorf("mkfcmap error: %v", err)
	}

	fmapping := svc.FlashCopyMappingStart{ID: fcmapping.Name, Prep: true, Restore: true}
	if err := svcclient.Startfcmap(fmapping); err != nil {
		return nil, "", fmt.Errorf("startfcmap error: %v", err)
	}

	mapping := svc.VolumeHostMap{Host: targetHost.ID, Force: true, VDisk: volume.Name}
	if err := svcclient.Mkvdiskhostmap(mapping); err != nil {
		return nil, "", fmt.Errorf("mkvdiskhostmap error: %v", err)
	}

	return targetVol, selectedViosName, nil
}

func identifyFreeVolume(restClient *hmc.HmcRestClient, viosUUID string, viosName string, VdiskUID string, verbose bool) (string, error) {
	pvList, err := restClient.GetFreePhyVolume(viosUUID, verbose)
	if err != nil {
		pvList = []hmc.PhysicalVolume{}
	}

	for _, pv := range pvList {
		if strings.Contains(pv.VolumeUniqueID, VdiskUID) {
			return pv.VolumeName, nil
		}
	}
	if len(pvList) == 0 {
		return "", fmt.Errorf("no free physical volumes found on VIOS %s", viosName)
	}
	return "", fmt.Errorf("volume with UID %s not found on VIOS", VdiskUID)
}
