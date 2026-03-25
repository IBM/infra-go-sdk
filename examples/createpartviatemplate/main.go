package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/beevik/etree"
	hmc "github.com/sudeeshjohn/powerhmc-go"
	svc "github.com/sudeeshjohn/svc-go-sdk"
)

func main() {
	// --- Command-line flags with hardcoded defaults ---
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("username", "REDACTED_HMC_USER<==", "Username")
	password := flag.String("password", "REDACTED_HMC_PASS<==", "Password")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	osType := flag.String("os-type", "linux", "OS type (aix, linux, aix_linux, ibmi)")
	systemName := flag.String("system-name", "LTC09U31-ZZ", "Managed system name")

	// --- SVC Configuration Flags ---
	svcIP := flag.String("svc-ip", "192.0.2.8", "SVC IP address")
	svcUser := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC Username")
	svcPass := flag.String("svc-pass", "REDACTED_HMC_PASS<==", "SVC Password")
	baseImageName := flag.String("base-image", "image-ibm-default-centos-10", "Base image name for FlashCopy")

	flag.Parse()

	if *verbose {
		log.Println("======================================================")
		log.Println(" Starting PowerVM LPAR Creation & SAN Provisioning")
		log.Println("======================================================")
	}

	// --- 1. Validate OS Type & Determine Reference Template immediately ---
	var referenceTemplate string
	if *systemName != "" && *osType != "" {
		if *verbose {
			log.Printf("[INIT] Validating OS type: %s", *osType)
		}
		switch *osType {
		case "aix", "linux", "aix_linux":
			referenceTemplate = "QuickStart_lpar_rpa_2"
		case "ibmi":
			referenceTemplate = "QuickStart_lpar_IBMi_2"
		default:
			log.Fatalf("[INIT] Error: invalid os-type: %s (must be aix, linux, aix_linux, or ibmi)", *osType)
		}
		if *verbose {
			log.Printf("[INIT] Selected Reference Template: %s", referenceTemplate)
		}
	}

	// --- Initialize & Authenticate HMC ---
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if *verbose {
		log.Printf("[HMC-AUTH] Attempting to log on to HMC at %s with username %s", *hmcIP, *username)
	}
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("[HMC-AUTH] Logon failed: %v", err)
	}
	defer func() {
		if *verbose {
			log.Printf("[HMC-AUTH] Initiating HMC Logoff sequence...")
		}
		if err := restClient.Logoff(); err != nil {
			log.Printf("[HMC-AUTH] Logoff failed: %v", err)
		} else if *verbose {
			log.Println("[HMC-AUTH] Logged off successfully")
		}
	}()

	// --- Main Partition Creation Workflow ---
	if *systemName != "" && *osType != "" {
		systemUUID := resolveSystemUUID(restClient, *systemName, *verbose)
		configDict := buildLparConfigDict(*verbose)

		// 2. Check if LPAR already exists
		ensureLparDoesNotExist(restClient, systemUUID, configDict["vm_name"], *verbose)

		// 3. Prepare the Partition Template
		tempUUID, tempTemplateName, tempTemplateDoc, err := prepareLparTemplate(restClient, referenceTemplate, configDict, *verbose)
		if err != nil {
			log.Fatalf("[HMC] Template preparation failed: %v", err)
		}

		// 4. Discover VIOS WWPNs from HMC
		if *verbose {
			log.Println("[HMC] Starting VIOS WWPN discovery...")
		}
		viosWwpnMap, viosUuidMap := getViosWwpnMap(restClient, systemUUID, *verbose)

		// 5. Provision SVC Storage
		if *verbose {
			log.Println("[SVC] Starting SVC storage provisioning...")
		}
		targetVol, selectedViosName := provisionSVCStorage(*svcIP, *svcUser, *svcPass, *baseImageName, viosWwpnMap, *verbose)

		// 6. Configure VSCSI and update the template (with cached VIOS UUIDs)
		configureVSCSI(restClient, systemUUID, tempUUID, targetVol, tempTemplateDoc, selectedViosName, viosUuidMap, *verbose)

		// 7. Deploy, Start, and Cleanup
		deployAndStartPartition(restClient, systemUUID, tempUUID, tempTemplateName, configDict, *osType, *verbose)

		if *verbose {
			log.Println("======================================================")
			log.Printf(" ✅ Workflow Complete! LPAR '%s' is up and running.", configDict["vm_name"])
			log.Println("======================================================")
		}
	}
}

// =========================================================================
// WORKFLOW HELPER FUNCTIONS
// =========================================================================

func resolveSystemUUID(restClient *hmc.HmcRestClient, systemName string, verbose bool) string {
	if verbose {
		log.Printf("[HMC] Resolving Managed System UUID for name: %s", systemName)
	}
	
	// Use the faster GetManagedSystemQuickAll (JSON) instead of GetManagedSystemByName (XML)
	systems, err := restClient.GetManagedSystemQuickAll(verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to get managed systems: %v", err)
	}
	
	// Find the system by name
	for _, system := range systems {
		if system.SystemName == systemName {
			if verbose {
				log.Printf("[HMC] Successfully resolved Managed System UUID: %s", system.UUID)
				log.Printf("[HMC] Maximum Partitions allowed for system %s: %d", system.UUID, system.MaximumPartitions)
			}
			return system.UUID
		}
	}
	
	log.Fatalf("[HMC] Given system '%s' is not present", systemName)
	return ""
}

func buildLparConfigDict(verbose bool) map[string]string {
	if verbose {
		log.Println("[CONFIG] Building LPAR configuration dictionary with default values...")
	}
	proc, procUnit, mem, maxVirtualSlots := 2, 2, 2048, 50
	
	configDict := make(map[string]string)
	configDict["vm_name"] = "test-test-test"
	configDict["proc"] = strconv.Itoa(proc)
	configDict["max_proc"] = strconv.Itoa(proc)
	configDict["min_proc"] = "1"
	configDict["proc_unit"] = strconv.Itoa(procUnit)
	configDict["max_proc_unit"] = strconv.Itoa(procUnit)
	configDict["min_proc_unit"] = ".1"
	configDict["mem"] = strconv.Itoa(mem)
	configDict["max_mem"] = strconv.Itoa(mem)
	configDict["min_mem"] = "1024"
	configDict["max_virtual_slots"] = strconv.Itoa(maxVirtualSlots)
	configDict["proc_mode"] = "uncapped"
	configDict["weight"] = "128"
	configDict["proc_comp_mode"] = "Default"
	configDict["shared_proc_pool"] = "0"
	
	if verbose {
		log.Printf("[CONFIG] LPAR Name: %s | CPU: %d | Mem: %dMB", configDict["vm_name"], proc, mem)
	}
	return configDict
}

func ensureLparDoesNotExist(restClient *hmc.HmcRestClient, systemUUID, vmName string, verbose bool) {
	if verbose {
		log.Printf("[HMC] Verifying LPAR name '%s' is unique on system...", vmName)
	}
	existingUUID, _, err := restClient.GetLogicalPartition(systemUUID, vmName, "", verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to check for existing LPAR: %v", err)
	}
	if existingUUID != "" {
		log.Fatalf("[HMC] Error: LPAR with name '%s' already exists (UUID: %s)", vmName, existingUUID)
	}
	if verbose {
		log.Printf("[HMC] Validation passed: LPAR name '%s' is available.", vmName)
	}
}

func prepareLparTemplate(restClient *hmc.HmcRestClient, referenceTemplate string, configDict map[string]string, verbose bool) (string, string, *etree.Element, error) {
	if verbose {
		log.Printf("[HMC-TMPL] Verifying existence of reference template: %s", referenceTemplate)
	}
	if _, err := restClient.GetPartitionTemplateID(referenceTemplate, verbose); err != nil {
		return "", "", nil, fmt.Errorf("reference template '%s' not found on the HMC: %v", referenceTemplate, err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	tempTemplateName := fmt.Sprintf("hmctool_powervm_create_%04d", rng.Intn(9000)+1000)

	if verbose {
		log.Printf("[HMC-TMPL] Copying reference template to temporary template: %s", tempTemplateName)
	}
	if err := restClient.CopyPartitionTemplate(referenceTemplate, tempTemplateName, verbose); err != nil {
		return "", "", nil, fmt.Errorf("failed to copy template from %s to %s: %v", referenceTemplate, tempTemplateName, err)
	}
	if verbose {
		log.Printf("[HMC-TMPL] Successfully copied template.")
	}

	if verbose {
		log.Printf("[HMC-TMPL] Retrieving XML structure for template: %s", tempTemplateName)
	}
	tempTemplateDoc, err := restClient.GetPartitionTemplate("", tempTemplateName, verbose)
	if err != nil || tempTemplateDoc == nil {
		return "", "", nil, fmt.Errorf("failed to retrieve temporary template %s: %v", tempTemplateName, err)
	}
	
	atomIDs := tempTemplateDoc.FindElements("//AtomID")
	if len(atomIDs) == 0 { 
		return "", "", nil, fmt.Errorf("AtomID not found for temporary template") 
	}
	tempUUID := atomIDs[0].Text()
	if verbose {
		log.Printf("[HMC-TMPL] Temporary Template UUID resolved: %s", tempUUID)
	}

	if verbose {
		log.Printf("[HMC-TMPL] Injecting target LPAR configurations into XML DOM...")
	}
	if err := restClient.UpdateLparNameAndIDToDom(tempTemplateDoc, configDict); err != nil {
		return "", "", nil, fmt.Errorf("failed to update template XML name/ID: %v", err)
	}
	if err := restClient.UpdateProcMemSettingsToDom(tempTemplateDoc, configDict); err != nil {
		return "", "", nil, fmt.Errorf("failed to update processor and memory settings: %v", err)
	}
	
	virtNetworkConfigs := []hmc.VirtualNetworkConfig{{NetworkName: "VNET0", SlotNumber: 49, VirtualSlotNumber: 49}}
	if err := restClient.UpdateVirtualNWSettingsToDom(tempTemplateDoc, virtNetworkConfigs); err != nil {
		return "", "", nil, fmt.Errorf("failed to update virtual network settings: %v", err)
	}

	if verbose {
		log.Printf("[HMC-TMPL] XML DOM successfully updated for template: %s", tempTemplateName)
	}
	return tempUUID, tempTemplateName, tempTemplateDoc, nil
}

func getViosWwpnMap(restClient *hmc.HmcRestClient, systemUUID string, verbose bool) (map[string][]string, map[string]string) {
	if verbose {
		log.Printf("[HMC] Fetching all Virtual I/O Servers for system UUID: %s to discover WWPNs and UUIDs...", systemUUID)
	}
	viosList, err := restClient.GetVirtualIOServers(systemUUID, verbose)
	if err != nil {
		log.Fatalf("[HMC] Failed to fetch VIOS details: %v", err)
	}

	viosWwpnMap := make(map[string][]string)
	viosUuidMap := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process each VIOS in parallel
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
				if verbose {
					log.Printf("[HMC] Discovered VIOS '%s' (UUID: %s) with %d Fibre Channel WWPN(s).", v.PartitionName, v.UUID, len(wwpns))
				}
			} else if verbose {
				log.Printf("[HMC] Skipped VIOS '%s' (No FC WWPNs found).", v.PartitionName)
			}
		}(i)
	}

	wg.Wait()

	if len(viosWwpnMap) == 0 {
		log.Fatalf("[HMC] Critical Error: No Fibre Channel WWPNs found on any VIOS on this system.")
	}
	return viosWwpnMap, viosUuidMap
}

func provisionSVCStorage(svcIP, svcUser, svcPass, baseImageName string, viosWwpnMap map[string][]string, verbose bool) (*svc.Vdisk, string) {
	if verbose {
		log.Printf("[SVC] Connecting to SVC Cluster at %s...", svcIP)
	}
	svcclient := svc.NewClient(svcIP, svcUser, svcPass).WithTLSInsecure()
	if err := svcclient.Authenticate(); err != nil {
		log.Fatalf("[SVC] Auth error: %v", err)
	}
	if verbose {
		log.Printf("[SVC] Authentication Successful.")
	}

	var selectedViosName string
	var selectedHostName string
	var selectedWWPNs []string
	hostExists := false

	if verbose {
		log.Println("[SVC] Cross-referencing HMC VIOS WWPNs against SVC Hosts...")
	}
	
	// --- 1. Optimized: Fetch fabric logins once and check all VIOS in parallel ---
	fabricLogins, err := svcclient.Lsfabric()
	if err != nil {
		log.Fatalf("[SVC] Failed to fetch fabric logins: %v", err)
	}
	
	// Build a map of WWPN -> HostName for fast lookup
	wwpnToHostMap := make(map[string]string)
	for _, login := range fabricLogins {
		upperWWPN := strings.ToUpper(login.RemoteWWPN)
		wwpnToHostMap[upperWWPN] = login.HostName
	}
	
	// Check all VIOS WWPNs against the cached fabric data
	for viosName, wwpns := range viosWwpnMap {
		for _, wwpn := range wwpns {
			upperWWPN := strings.ToUpper(wwpn)
			if hostName, found := wwpnToHostMap[upperWWPN]; found {
				if verbose {
					log.Printf("[SVC] ✅ Match Found! VIOS '%s' is mapped to SVC Host '%s' via WWPN %s", viosName, hostName, wwpn)
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

	// --- 2. Fallback: Create new host if none exist ---
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

		if verbose {
			log.Printf("[SVC] Creating new SVC host '%s' using WWPNs: %v", selectedHostName, selectedWWPNs)
		}
		newHost := svc.Host{
			Name:     selectedHostName,
			Fcwwpn:   selectedWWPNs,
			Type:     "generic",
			Protocol: "scsi",
		}
		if err := svcclient.Mkhost(newHost); err != nil && !strings.Contains(err.Error(), "already exists") {
			log.Fatalf("[SVC] Mkhost error: %v", err)
		}
		if verbose {
			log.Printf("[SVC] Host '%s' created successfully.", selectedHostName)
		}
	}

	// Retrieve final SVC host ID
	target_host, err := svcclient.LshostByTarget(selectedHostName)
	if err != nil {
		log.Fatalf("[SVC] Error finding host %s: %v", selectedHostName, err)
	}
	finalHostID := target_host.ID

	// --- 3. Create Volume ---
	if verbose {
		log.Printf("[SVC] Provisioning new VDisk (Volume)...")
	}
	volume := svc.Volume{
		Name: "test_volume2", MdiskGrp: "0", Size: 120, Unit: "gb",
		RSize: "2%", Warning: "80%", AutoExpand: true, GrainSize: 256,
	}
	if err := svcclient.Mkvdisk(volume); err != nil {
		log.Fatalf("[SVC] Mkvdisk error: %v", err)
	}
	if verbose {
		log.Printf("[SVC] Successfully created volume: %s", volume.Name)
	}

	if verbose {
		log.Printf("[SVC] Locating Source Base Image: %s", baseImageName)
	}
	sourceVol, _ := svcclient.LsVdiskByName(baseImageName)
	targetVol, _ := svcclient.LsVdiskByName(volume.Name)

	// --- 4. FlashCopy ---
	if verbose {
		log.Printf("[SVC] Setting up FlashCopy Mapping from %s -> %s", sourceVol.Name, targetVol.Name)
	}
	fcmapping := svc.FlashCopyMapping{
		Name: "test_fcmap", Source: sourceVol.ID, Target: targetVol.ID,
		CopyRate: 150, GrainSize: 256, Incremental: true, AutoDelete: true,
	}
	if err := svcclient.Mkfcmap(fcmapping); err != nil {
		log.Fatalf("[SVC] Mkfcmap error: %v", err)
	}

	if verbose {
		log.Printf("[SVC] Starting FlashCopy operation...")
	}
	fmapping := svc.FlashCopyMappingStart{ID: fcmapping.Name, Prep: true, Restore: true}
	if err := svcclient.Startfcmap(fmapping); err != nil {
		log.Fatalf("[SVC] Startfcmap error: %v", err)
	}

	// --- 5. Map to Host ---
	if verbose {
		log.Printf("[SVC] Mapping Target Volume '%s' to Host '%s'", volume.Name, finalHostID)
	}
	mapping := svc.VolumeHostMap{Host: finalHostID, Force: true, VDisk: volume.Name}
	if err := svcclient.Mkvdiskhostmap(mapping); err != nil {
		log.Fatalf("[SVC] Mkvdiskhostmap error: %v", err)
	}

	return targetVol, selectedViosName
}

func configureVSCSI(restClient *hmc.HmcRestClient, systemUUID, tempUUID string, targetVol *svc.Vdisk, tempTemplateDoc *etree.Element, viosName string, viosUuidMap map[string]string, verbose bool) {
	if verbose {
		log.Printf("[HMC-VSCSI] Beginning VSCSI Configuration on VIOS: %s", viosName)
	}
	
	volumeConfigs := []hmc.VolumeConfig{
		{ViosName: viosName, VolumeName: targetVol.Name},
	}

	vscsiClientsPayload := ""
	for _, volConfig := range volumeConfigs {
		// Use cached VIOS UUID instead of calling GetViosID
		viosUUID, exists := viosUuidMap[volConfig.ViosName]
		if !exists {
			log.Fatalf("[HMC-VSCSI] VIOS UUID not found in cache for: %s", volConfig.ViosName)
		}
		if verbose {
			log.Printf("[HMC-VSCSI] Using cached UUID for VIOS '%s': %s", volConfig.ViosName, viosUUID)
		}
		
		if verbose {
			log.Printf("[HMC-VSCSI] Running ConfigDevice (cfgdev) on VIOS %s to scan for new SVC disks...", volConfig.ViosName)
		}
		restClient.ConfigDevice(viosUUID, "", verbose)
		
		if verbose {
			log.Printf("[HMC-VSCSI] Correlating SVC VdiskUID (%s) to an HMC Physical Volume...", targetVol.VdiskUID)
		}
		pv, err := identifyFreeVolume(restClient, viosUUID, volConfig, targetVol.VdiskUID, verbose)
		if err != nil { log.Fatalf("[HMC-VSCSI] Failed to identify free volume: %v", err) }
		
		if verbose {
			log.Printf("[HMC-VSCSI] Matched VdiskUID to HMC Volume: %s. Generating XML Payload.", pv)
		}
		vscsiClientsPayload += hmc.AddVSCSIPayload(volConfig, pv, verbose)
	}

	if vscsiClientsPayload != "" {
		if verbose {
			log.Printf("[HMC-VSCSI] Injecting VSCSI Payload into Template XML...")
		}
		if err := hmc.AddVSCSI(tempTemplateDoc, vscsiClientsPayload); err != nil {
			log.Fatalf("[HMC-VSCSI] Failed to add VSCSI to template XML: %v", err)
		}
		
		if verbose {
			log.Printf("[HMC-VSCSI] Pushing updated Template XML back to HMC...")
		}
		if err := restClient.UpdatePartitionTemplate(tempUUID, tempTemplateDoc, verbose); err != nil {
			log.Fatalf("[HMC-VSCSI] Failed to update partition template with VSCSI: %v", err)
		}
		if verbose {
			log.Printf("[HMC-VSCSI] VSCSI Configuration successfully saved to Template.")
		}
	}
}

func deployAndStartPartition(restClient *hmc.HmcRestClient, systemUUID, tempUUID, tempTemplateName string, configDict map[string]string, osType string, verbose bool) {
	if verbose {
		log.Printf("[HMC-DEPLOY] Transforming Partition Template (UUID: %s) for System Deployment...", tempUUID)
	}
	if _, err := restClient.TransformPartitionTemplate(tempUUID, systemUUID, verbose); err != nil {
		log.Fatalf("[HMC-DEPLOY] Template transform failed: %v", err)
	}
	
	if verbose {
		log.Printf("[HMC-DEPLOY] Running pre-deployment Check on Template...")
	}
	if _, err := restClient.CheckPartitionTemplate(tempTemplateName, systemUUID, verbose); err != nil {
		log.Fatalf("[HMC-DEPLOY] Template check failed: %v", err)
	}

	if verbose {
		log.Printf("[HMC-DEPLOY] Executing Partition Deployment Job...")
	}
	partUUID, err := restClient.DeployPartitionTemplate(tempUUID, systemUUID, verbose)
	if err != nil {
		log.Fatalf("[HMC-DEPLOY] Failed to deploy partition: %v", err)
	}
	if verbose {
		log.Printf("[HMC-DEPLOY] Partition deployed successfully. New LPAR UUID: %s", partUUID)
		log.Printf("[HMC-DEPLOY] Fetching default Partition Profile UUID...")
	}

	// Fetch detailed LPAR information to get the default profile UUID
	lparDetails, err := restClient.GetLogicalPartitionDetailed(partUUID, verbose)
	if err != nil {
		log.Fatalf("[HMC-DEPLOY] Failed to get LPAR details: %v", err)
	}

	// Extract profile UUID from the AssociatedPartitionProfile href
	profileHref := lparDetails.AssociatedPartitionProfile.Href
	if profileHref == "" {
		log.Fatalf("[HMC-DEPLOY] No associated partition profile found for LPAR")
	}
	
	// Extract UUID from href (last 36 characters)
	if len(profileHref) < 36 {
		log.Fatalf("[HMC-DEPLOY] Invalid profile href format: %s", profileHref)
	}
	profileUUID := profileHref[len(profileHref)-36:]

	if verbose {
		log.Printf("[HMC-DEPLOY] Using default profile '%s' (UUID: %s)", lparDetails.DefaultProfileName, profileUUID)
		log.Printf("[HMC-DEPLOY] Powering on LPAR (UUID: %s) with Profile: %s", partUUID, profileUUID)
	}
	// Create PowerOnOptions
	options := &hmc.PowerOnOptions{
		ProfileUUID: profileUUID,
		Keylock:     "manual",
		OSType:      osType,
	}
	
	if _, err := restClient.PowerOnPartition(partUUID, options, verbose); err != nil {
		log.Fatalf("[HMC-DEPLOY] Failed to PowerOn Partition: %v", err)
	}
	if verbose {
		log.Printf("[HMC-DEPLOY] LPAR is booting up.")
	}

	if verbose {
		log.Printf("[HMC-DEPLOY] Cleaning up temporary Partition Template: %s", tempTemplateName)
	}
	if err := restClient.DeletePartitionTemplate(tempTemplateName, verbose); err != nil {
		log.Fatalf("[HMC-DEPLOY] Failed to Delete temporary Partition Template: %v", err)
	}

	if verbose {
		log.Printf("[HMC-DEPLOY] Waiting 10 seconds before initiating final Restart sequence...")
	}
	time.Sleep(10 * time.Second)
	
	if verbose {
		log.Printf("[HMC-DEPLOY] Triggering 'Immediate' PowerOff/Restart on LPAR...")
	}
	if _, err := restClient.PowerOffPartition(partUUID, "Immediate", true, verbose); err != nil {
		log.Fatalf("[HMC-DEPLOY] Failed to Restart Partition: %v", err)
	}
}

// =========================================================================
// PRE-EXISTING WRAPPER FUNCTIONS (Preserved Functionality)
// =========================================================================

func identifyFreeVolume(restClient *hmc.HmcRestClient, viosUUID string, volConfig hmc.VolumeConfig, VdiskUID string, verbose bool) (string, error) {
	viosName := volConfig.ViosName
	if verbose {
		log.Printf("[HMC-UTIL] Identifying free volume on VIOS '%s' (UUID: %s) matching UID '%s'", viosName, viosUUID, VdiskUID)
	}

	pvList, err := restClient.GetFreePhyVolume(viosUUID, verbose)
	if err != nil { pvList = []hmc.PhysicalVolume{} }

	for _, pv := range pvList {
		if strings.Contains(pv.VolumeUniqueID, VdiskUID) {
			if verbose {
				log.Printf("[HMC-UTIL] Match found! UID %s maps to Physical Volume %s", VdiskUID, pv.VolumeName)
			}
			return pv.VolumeName, nil
		}
	}
	if len(pvList) == 0 { return "", fmt.Errorf("no free physical volumes found on VIOS %s", viosName) }
	return "", fmt.Errorf("volume %s not found on VIOS", volConfig.VolumeName)
}