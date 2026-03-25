package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
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
	lparName := flag.String("lpar-name", "Go_LPAR_100", "Name for the new LPAR")
	osType := flag.String("os-type", "AIX/Linux", "OS type (AIX/Linux, OS400, Virtual IO Server)")

	// Networking Config
	vswitchName := flag.String("vswitch-name", "ETHERNET0(Default)", "Name of the Virtual Switch")
	vlanID := flag.Int("vlan-id", 1337, "VLAN ID for the Client Network Adapter")

	// SVC Config (for Physical Disk)
	svcIP := flag.String("svc-ip", "192.0.2.8", "SVC IP address")
	svcUser := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC Username")
	svcPass := flag.String("svc-pass", "REDACTED_HMC_PASS<==", "SVC Password")
	baseImageName := flag.String("base-image", "image-ibm-default-centos-10", "Base image name for FlashCopy")

	// Virtual Disk Config
	targetVios := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS for virtual disk (Leave empty for auto-select)")
	targetVg := flag.String("vg-name", "auto_vg01", "Target Volume Group (Leave empty for auto-select)")
	virtualDiskName := flag.String("vdisk-name", "", "Name of the Virtual Disk (auto-generated if empty)")
	virtualDiskSize := flag.Int("vdisk-size", 51200, "Size of the Virtual Disk in Megabytes")

	// Optical Media Config
	mediaNamesStr := flag.String("media-names", "test_iso", "Comma-separated list of ISO files to map (e.g., 'rhel9.iso,aix73.iso'). Leave empty to skip optical mapping.")

	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	// Derive names from LPAR name with timestamp for uniqueness
	timestamp := time.Now().Unix() % 10000 // Last 4 digits of timestamp
	physicalVolumeName := fmt.Sprintf("%s_boot_%d", strings.ReplaceAll(*lparName, " ", "_"), timestamp)
	if *virtualDiskName == "" {
		// AIX/VIOS has 15-char limit for LV names, so use shorter format
		cleanLpar := strings.ReplaceAll(*lparName, " ", "_")
		if len(cleanLpar) > 8 {
			cleanLpar = cleanLpar[:8]
		}
		*virtualDiskName = fmt.Sprintf("%s_d%d", cleanLpar, timestamp)
	}

	log.Println("=========================================================================")
	log.Println(" 🚀 Starting Multi-Storage PowerVM LPAR Provisioning")
	log.Println("    - Physical SAN Disk (via SVC FlashCopy)")
	log.Println("    - Virtual Disk (Native VIOS LV)")
	if *mediaNamesStr != "" {
		log.Println("    - Virtual Optical Media (ISO files)")
	}
	log.Println("=========================================================================")

	// =========================================================================
	// 1. PARALLEL AUTHENTICATION (HMC + SVC)
	// =========================================================================
	log.Println("")
	log.Println("🔀 Phase 1: Parallel Authentication (HMC || SVC)...")

	var restClient *hmc.HmcRestClient
	var svcclient *svc.Client
	var wg sync.WaitGroup
	var hmcErr, svcErr error

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
	log.Println("🔀 Phase 2: 3-Way Parallel Operations (LPAR || VIOS || vSwitch)...")

	var lparDetails *hmc.LogicalPartitionDetailed
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
			Name:             *lparName,
			OsType:           *osType,
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
		log.Printf("[Thread-LPAR] ✅ LPAR Created! UUID: %s", lparDetails.MetadataID)
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
	// 4. ATTACH NETWORK ADAPTER
	// =========================================================================
	lparUUID := lparDetails.MetadataID
	
	log.Println("")
	log.Printf("[HMC] Phase 3: Attaching VLAN %d to LPAR...", *vlanID)
	_, err := restClient.CreateClientNetworkAdapter(sysUUID, lparUUID, vswitchUUID, *vlanID, *verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to add network adapter: %v", err)
	}
	log.Printf("[HMC] ✅ Network Adapter Attached.")

	// =========================================================================
	// 5. PARALLEL STORAGE PROVISIONING: PHYSICAL SAN || VIRTUAL DISK
	// =========================================================================
	log.Println("")
	log.Println("🔀 Phase 4: Parallel Storage Provisioning (Physical SAN || Virtual Disk)...")

	type physicalStorageResult struct {
		targetVol        *svc.Vdisk
		selectedViosName string
		selectedViosUUID string
		diskName         string
	}

	type virtualStorageResult struct {
		viosUUID string
		viosName string
	}

	physicalResCh := make(chan physicalStorageResult, 1)
	physicalErrCh := make(chan error, 1)
	virtualResCh := make(chan virtualStorageResult, 1)
	virtualErrCh := make(chan error, 1)

	wg.Add(2)

	// Branch A: Provision Physical SAN Disk
	go func() {
		defer wg.Done()
		log.Println("[Branch-A] Starting SVC storage provisioning...")

		targetVol, selectedViosName, err := provisionSVCStorage(svcclient, *baseImageName, viosWwpnMap, physicalVolumeName, *verbose)
		if err != nil {
			physicalErrCh <- fmt.Errorf("failed to provision SVC storage: %v", err)
			return
		}

		selectedViosUUID := viosUuidMap[selectedViosName]

		log.Printf("[Branch-A] Running ConfigDevice (cfgdev) to scan for the new SVC LUN...")
		if err := restClient.ConfigDevice(selectedViosUUID, "", *verbose); err != nil {
			physicalErrCh <- fmt.Errorf("failed to run cfgdev: %v", err)
			return
		}

		log.Printf("[Branch-A] Locating new physical volume matching SVC UID: %s...", targetVol.VdiskUID)
		diskName, err := identifyFreeVolume(restClient, selectedViosUUID, selectedViosName, targetVol.VdiskUID, *verbose)
		if err != nil {
			physicalErrCh <- fmt.Errorf("failed to identify free volume: %v", err)
			return
		}
		log.Printf("[Branch-A] ✅ Matched SVC LUN to VIOS Disk: %s", diskName)

		physicalResCh <- physicalStorageResult{
			targetVol:        targetVol,
			selectedViosName: selectedViosName,
			selectedViosUUID: selectedViosUUID,
			diskName:         diskName,
		}
	}()

	// Branch B: Provision Virtual Disk
	go func() {
		defer wg.Done()
		log.Printf("[Branch-B] Discovering optimal Volume Group for %d MB virtual disk...", *virtualDiskSize)

		viosUUID, viosName, err := provisionVirtualDisk(restClient, *sysName, sysUUID, *virtualDiskName, *targetVios, *targetVg, *virtualDiskSize, *verbose)
		if err != nil {
			virtualErrCh <- fmt.Errorf("failed to provision virtual disk: %v", err)
			return
		}

		log.Printf("[Branch-B] ✅ Virtual Disk '%s' Provisioned on VIOS '%s'.", *virtualDiskName, viosName)
		virtualResCh <- virtualStorageResult{viosUUID: viosUUID, viosName: viosName}
	}()

	wg.Wait()

	// Check for errors
	var physicalStorage physicalStorageResult
	select {
	case err := <-physicalErrCh:
		log.Fatalf("❌ Physical Storage Branch Failed: %v", err)
	case physicalStorage = <-physicalResCh:
	}

	var virtualStorage virtualStorageResult
	select {
	case err := <-virtualErrCh:
		log.Fatalf("❌ Virtual Storage Branch Failed: %v", err)
	case virtualStorage = <-virtualResCh:
	}

	log.Println("✅ Both storage provisioning branches completed successfully")

	// =========================================================================
	// 6. MAP BOTH DISKS TO LPAR
	// =========================================================================
	log.Println("")
	log.Println("[HMC] Phase 5: Mapping both disks to LPAR...")

	// Map Physical Disk
	log.Printf("[HMC] Attaching Physical Disk '%s' to LPAR '%s'...", physicalStorage.diskName, *lparName)
	mappingUUID1, err := restClient.CreatePhysicalVolumeMap(sysUUID, physicalStorage.selectedViosUUID, lparUUID, []string{physicalStorage.diskName}, *verbose)
	if err != nil {
		log.Fatalf("[HMC] Physical Storage Mapping Failed: %v", err)
	}

	if mappingUUID1 == "SUCCESS_WITH_RMC_WARNING" {
		log.Printf("[HMC] ✅ Physical disk mapped successfully! (Ignored expected RMC warning for offline LPAR)")
	} else {
		log.Printf("[HMC] ✅ Physical disk mapped successfully!")
	}

	// Map Virtual Disk
	log.Printf("[HMC] Attaching Virtual Disk '%s' to LPAR '%s'...", *virtualDiskName, *lparName)
	mappingUUID2, err := restClient.CreateVirtualDiskMaps(sysUUID, virtualStorage.viosUUID, lparUUID, []string{*virtualDiskName}, *verbose)
	if err != nil {
		log.Fatalf("[HMC] Virtual Storage Mapping Failed: %v", err)
	}

	if mappingUUID2 == "SUCCESS_WITH_RMC_WARNING" {
		log.Printf("[HMC] ✅ Virtual disk mapped successfully! (Ignored expected RMC warning for offline LPAR)")
	} else {
		log.Printf("[HMC] ✅ Virtual disk mapped successfully!")
	}

	// =========================================================================
	// 6. MAP OPTICAL MEDIA (OPTIONAL)
	// =========================================================================
	var opticalMediaCount int
	if *mediaNamesStr != "" {
		log.Println("")
		log.Println("[HMC] Phase 5.5: Mapping Virtual Optical Media...")
		
		// Parse comma-separated media names
		mediaNames := strings.Split(*mediaNamesStr, ",")
		for i := range mediaNames {
			mediaNames[i] = strings.TrimSpace(mediaNames[i])
		}
		
		log.Printf("[HMC] Attaching %d Virtual Optical Media to LPAR '%s'...", len(mediaNames), *lparName)
		mappingUUID3, err := restClient.CreateVirtualOpticalMaps(sysUUID, virtualStorage.viosUUID, lparUUID, mediaNames, *verbose)
		if err != nil {
			log.Printf("[HMC] ⚠️  Warning: Virtual Optical Media mapping failed: %v", err)
		} else {
			if mappingUUID3 == "SUCCESS_WITH_RMC_WARNING" {
				log.Printf("[HMC] ✅ Virtual optical media mapped successfully! (Ignored expected RMC warning for offline LPAR)")
			} else {
				log.Printf("[HMC] ✅ Virtual optical media mapped successfully!")
			}
			opticalMediaCount = len(mediaNames)
		}
	}

	// =========================================================================
	// 7. SAVE CONFIGURATION & POWER ON THE LPAR
	// =========================================================================
	// Use the default profile name from the LPAR details
	profileName := lparDetails.DefaultProfileName
	log.Println("")
	log.Printf("[HMC] Phase 6: Saving active configuration to profile '%s'...", profileName)
	err = restClient.SaveCurrentLparConfig(lparUUID, profileName, true, *verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to save LPAR configuration: %v", err)
	}
	log.Printf("[HMC] ✅ Configuration permanently saved to profile.")

	log.Println("")
	log.Printf("[HMC] Phase 7: Powering on LPAR '%s'...", *lparName)

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
	log.Printf(" 🎉 SUCCESS: Multi-Storage PowerVM Provisioning Complete!")
	log.Printf("    - LPAR Name      : %s is BOOTING", *lparName)
	log.Printf("    - Network        : Attached to %s (VLAN %d)", *vswitchName, *vlanID)
	log.Printf("    - Physical Disk  : SVC Vol '%s' (%s) via %s", physicalStorage.targetVol.Name, physicalStorage.diskName, physicalStorage.selectedViosName)
	log.Printf("    - Virtual Disk   : Native LV '%s' via %s", *virtualDiskName, virtualStorage.viosName)
	if opticalMediaCount > 0 {
		log.Printf("    - Optical Media  : %d ISO(s) mounted via %s", opticalMediaCount, virtualStorage.viosName)
	}
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
	_, existingUUID, err := restClient.GetLogicalPartitionByName(systemUUID, vmName, false)
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
		if hostExists {
			break
		}
	}

	if !hostExists {
		if verbose {
			log.Println("[SVC] ⚠️ No matching SVC host found. Preparing to create a new host mapping...")
		}
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
		Name:       volumeName,
		MdiskGrp:   "0",
		Size:       120,
		Unit:       "gb",
		RSize:      "2%",
		Warning:    "80%",
		AutoExpand: true,
		GrainSize:  256,
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
		Name:        fmt.Sprintf("fcmap_%d", time.Now().Unix()),
		Source:      sourceVol.ID,
		Target:      targetVol.ID,
		CopyRate:    150,
		GrainSize:   256,
		Incremental: true,
		AutoDelete:  true,
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

// provisionVirtualDisk performs Smart Capacity Discovery to find the best VG, then creates the disk.
func provisionVirtualDisk(restClient *hmc.HmcRestClient, sysName, sysUUID, diskName, targetVios, targetVg string, diskSizeMB int, verbose bool) (string, string, error) {
	requiredGB := float64(diskSizeMB) / 1024.0

	viosList, err := restClient.GetVirtualIOServersQuick(sysUUID, verbose)
	if err != nil || len(viosList) == 0 {
		return "", "", fmt.Errorf("failed to fetch VIOS instances for system")
	}

	var finalViosUUID, finalViosName, finalVgName string
	var usingRootVgFallback bool

	for _, vios := range viosList {
		// Filter by VIOS if provided
		if targetVios != "" && !strings.EqualFold(vios.PartitionName, targetVios) {
			continue
		}

		vgList, err := restClient.GetVolumeGroups(vios.UUID, verbose)
		if err != nil {
			continue
		}

		for _, vg := range vgList {
			// Filter by VG if provided
			if targetVg != "" && !strings.EqualFold(vg.GroupName, targetVg) {
				continue
			}

			// Ensure no naming collision exists in this VG
			collision := false
			for _, vd := range vg.VirtualDisks {
				if strings.EqualFold(vd.DiskName, diskName) {
					collision = true
					break
				}
			}
			if collision {
				continue
			}

			freeSpaceGB, parseErr := strconv.ParseFloat(vg.FreeSpace, 64)
			if parseErr != nil {
				continue
			}

			// Capacity Check
			if freeSpaceGB >= requiredGB {
				if targetVg == "" {
					// Smart selection: Avoid rootvg if possible
					if strings.ToLower(vg.GroupName) == "rootvg" {
						if finalVgName == "" {
							finalViosUUID, finalViosName, finalVgName = vios.UUID, vios.PartitionName, vg.GroupName
							usingRootVgFallback = true
						}
					} else {
						// Found a perfect Data VG match
						finalViosUUID, finalViosName, finalVgName = vios.UUID, vios.PartitionName, vg.GroupName
						usingRootVgFallback = false
						break
					}
				} else {
					// Explicit match requested
					finalViosUUID, finalViosName, finalVgName = vios.UUID, vios.PartitionName, vg.GroupName
					break
				}
			}
		}
		if finalVgName != "" && !usingRootVgFallback {
			break
		}
	}

	if finalVgName == "" {
		return "", "", fmt.Errorf("could not find a Volume Group with %.2f GB of free space", requiredGB)
	}

	// Create the disk via the Smart CLI Wrapper
	err = restClient.CreateVirtualDisk(sysName, finalViosUUID, finalViosName, finalVgName, diskName, diskSizeMB, verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to create Virtual Disk via CLI: %v", err)
	}

	return finalViosUUID, finalViosName, nil
}

// Made with Bob
