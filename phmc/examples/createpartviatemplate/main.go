package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/beevik/etree"
	hmc "github.com/IBM/infra-go-sdk/phmc"
	svc "github.com/IBM/infra-go-sdk/svc"
	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

func main() {
	// --- Command-line flags with hardcoded defaults ---
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("username", "", "Username")
	password := flag.String("password", "", "Password")
	osType := flag.String("os-type", "linux", "OS type (aix, linux, aix_linux, ibmi)")
	systemName := flag.String("system-name", "", "Managed system name")

	// --- SVC Configuration Flags ---
	svcIP := flag.String("svc-ip", "", "SVC IP address")
	svcUser := flag.String("svc-user", "", "SVC Username")
	svcPass := flag.String("svc-pass", "", "SVC Password")
	baseImageName := flag.String("base-image", "image-ibm-default-centos-10", "Base image name for FlashCopy")

	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel() // Automatically cleans up the timer/goroutine the second the function exits


	// --- 1. Validate OS Type & Determine Reference Template immediately ---
	var referenceTemplate string
	if *systemName != "" && *osType != "" {
		switch *osType {
		case "aix", "linux", "aix_linux":
			referenceTemplate = "QuickStart_lpar_rpa_2"
		case "ibmi":
			referenceTemplate = "QuickStart_lpar_IBMi_2"
		default:
			log.Fatalf("[INIT] Error: invalid os-type: %s (must be aix, linux, aix_linux, or ibmi)", *osType)
		}
	}

	// --- Initialize & Authenticate HMC ---
	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)
	if err := restClient.Login(context.Background(), *username, *password); err != nil {
		log.Fatalf("[HMC-AUTH] Logon failed: %v", err)
	}
	defer func() {
		if err := restClient.Logoff(context.Background()); err != nil {
			log.Printf("[HMC-AUTH] Logoff failed: %v", err)
			log.Println("[HMC-AUTH] Logged off successfully")
		}
	}()

	// --- Main Partition Creation Workflow ---
	if *systemName != "" && *osType != "" {
		systemUUID := resolveSystemUUID(restClient, *systemName)
		configDict := buildLparConfigDict()

		// 2. Check if LPAR already exists
		ensureLparDoesNotExist(restClient, systemUUID, configDict["vm_name"])

		// 3. Prepare the Partition Template
		tempUUID, tempTemplateName, tempTemplateDoc, err := prepareLparTemplate(restClient, referenceTemplate, configDict)
		if err != nil {
			log.Fatalf("[HMC] Template preparation failed: %v", err)
		}

		// 4. Discover VIOS WWPNs from HMC
		viosWwpnMap, viosUuidMap := getViosWwpnMap(restClient, systemUUID)

		// 5. Provision SVC Storage
		targetVol, selectedViosName := provisionSVCStorage(ctx, *svcIP, *svcUser, *svcPass, *baseImageName, viosWwpnMap)

		// 6. Configure VSCSI and update the template (with cached VIOS UUIDs)
		configureVSCSI(ctx,restClient, systemUUID, tempUUID, targetVol, tempTemplateDoc, selectedViosName, viosUuidMap)

		// 7. Deploy, Start, and Cleanup
		deployAndStartPartition(ctx, restClient, systemUUID, tempUUID, tempTemplateName, configDict, *osType)

	}
}

// =========================================================================
// WORKFLOW HELPER FUNCTIONS
// =========================================================================

func resolveSystemUUID(restClient *hmc.RestClient, systemName string) string {
	if false {
		log.Printf("Resolving Managed System UUID for name: %s: %v", systemName)
	}
	
	// Use the faster GetManagedSystemQuickAll (JSON) instead of GetManagedSystemByName (XML)
	systems, err := restClient.GetManagedSystemQuickAll(context.Background())
	if err != nil {
		log.Fatalf("[HMC] Failed to get managed systems: %v", err)
	}
	
	// Find the system by name
	for _, system := range systems {
		if system.SystemName == systemName {
			if false {
				log.Printf("Successfully resolved Managed System UUID: %s: %v", system.UUID)
				log.Printf("Maximum Partitions allowed for system %s: %d: system.UUID=%v", system.MaximumPartitions)
			}
			return system.UUID
		}
	}
	
	log.Fatalf("[HMC] Given system '%s' is not present", systemName)
	return ""
}

func buildLparConfigDict() map[string]string {
	if false {
		log.Println("[CONFIG] Building LPAR configuration dictionary with default values...")
	}
	proc, procUnit, mem, maxVirtualSlots := 2, 2, 65536, 50
	
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
	
	if false {
		log.Printf("LPAR Name: %s | CPU: %d | Mem: %dMB: %v", configDict["vm_name"], proc, mem)
	}
	return configDict
}

func ensureLparDoesNotExist(restClient *hmc.RestClient, systemUUID, vmName string) {
	if false {
		log.Printf("Verifying LPAR name '%s' is unique on system...: %v", vmName)
	}
	existingUUID, _, err := restClient.GetLogicalPartition(context.Background(), systemUUID, vmName, "")
	if err != nil {
		log.Fatalf("[HMC] Failed to check for existing LPAR: %v", err)
	}
	if existingUUID != "" {
		log.Fatalf("[HMC] Error: LPAR with name '%s' already exists (UUID: %s)", vmName, existingUUID)
	}
	if false {
		log.Printf("Validation passed: LPAR name '%s' is available.: %v", vmName)
	}
}

func prepareLparTemplate(restClient *hmc.RestClient, referenceTemplate string, configDict map[string]string) (string, string, *etree.Element, error) {
	if false {
		log.Printf("[HMC-TMPL] Verifying existence of reference template: %s", referenceTemplate)
	}
	if _, err := restClient.GetPartitionTemplateID(referenceTemplate); err != nil {
		return "", "", nil, fmt.Errorf("reference template '%s' not found on the HMC: %v", referenceTemplate, err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	tempTemplateName := fmt.Sprintf("hmctool_powervm_create_%04d", rng.Intn(9000)+1000)

	if false {
		log.Printf("[HMC-TMPL] Copying reference template to temporary template: %s", tempTemplateName)
	}
	if err := restClient.CopyPartitionTemplate(referenceTemplate, tempTemplateName); err != nil {
		return "", "", nil, fmt.Errorf("failed to copy template from %s to %s: %v", referenceTemplate, tempTemplateName, err)
	}
	if false {
		log.Printf("[HMC-TMPL] Successfully copied template.")
	}

	if false {
		log.Printf("[HMC-TMPL] Retrieving XML structure for template: %s", tempTemplateName)
	}
	tempTemplateDoc, err := restClient.GetPartitionTemplate("", tempTemplateName)
	if err != nil || tempTemplateDoc == nil {
		return "", "", nil, fmt.Errorf("failed to retrieve temporary template %s: %v", tempTemplateName, err)
	}
	
	atomIDs := tempTemplateDoc.FindElements("//AtomID")
	if len(atomIDs) == 0 { 
		return "", "", nil, fmt.Errorf("AtomID not found for temporary template") 
	}
	tempUUID := atomIDs[0].Text()
	if false {
		log.Printf("[HMC-TMPL] Temporary Template UUID resolved: %s", tempUUID)
	}

	if false {
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

	if false {
		log.Printf("[HMC-TMPL] XML DOM successfully updated for template: %s", tempTemplateName)
	}
	return tempUUID, tempTemplateName, tempTemplateDoc, nil
}

func getViosWwpnMap(restClient *hmc.RestClient, systemUUID string) (map[string][]string, map[string]string) {
	if false {
		log.Printf("Fetching all Virtual I/O Servers for system UUID: %s to discover WWPNs and UUIDs...: %v", systemUUID)
	}
	viosList, err := restClient.GetVirtualIOServers(systemUUID)
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
			// Extract WWPNs from nested structure
			for _, profileSlot := range v.PartitionIOConfiguration.ProfileIOSlots {
				fcAdapter := profileSlot.AssociatedIOSlot.RelatedIOAdapter.PhysicalFibreChannelAdapter
				for _, port := range fcAdapter.PhysicalFibreChannelPorts {
					if port.WWPN != "" {
						wwpns = append(wwpns, strings.ToUpper(port.WWPN))
					}
				}
			}
			if len(wwpns) > 0 {
				mu.Lock()
				viosWwpnMap[v.PartitionName] = wwpns
				viosUuidMap[v.PartitionName] = v.PartitionUUID
				mu.Unlock()
				if false {
					log.Printf("Discovered VIOS '%s' (UUID: %s) with %d Fibre Channel WWPN(s).: %v", v.PartitionName, v.PartitionUUID, len(wwpns))
				}
			} else if false {
				log.Printf("Skipped VIOS '%s' (No FC WWPNs found).: %v", v.PartitionName)
			}
		}(i)
	}

	wg.Wait()

	if len(viosWwpnMap) == 0 {
		log.Fatalf("[HMC] Critical Error: No Fibre Channel WWPNs found on any VIOS on this system.")
	}
	return viosWwpnMap, viosUuidMap
}

func provisionSVCStorage(ctx context.Context, svcIP, svcUser, svcPass, baseImageName string, viosWwpnMap map[string][]string) (*svc.Vdisk, string) {
	if false {
		log.Printf("Connecting to SVC Cluster at %s...: %v", svcIP)
	}
	svcclient := svc.NewClient(svcIP, svcUser, svcPass).WithTLSInsecure()
	if err := svcclient.Authenticate(ctx); err != nil {
		log.Fatalf("[SVC] Auth error: %v", err)
	}
	if false {
		log.Println("Authentication Successful.")
	}

	var selectedViosName string
	var selectedHostName string
	var selectedWWPNs []string
	hostExists := false

	if false {
		log.Println("[SVC] Cross-referencing HMC VIOS WWPNs against SVC Hosts...")
	}
	
	// --- 1. Optimized: Fetch fabric logins once and check all VIOS in parallel ---
	fabricLogins, err := svcclient.Lsfabric(ctx)
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
				if false {
					log.Printf("✅ Match Found! VIOS '%s' is mapped to SVC Host '%s' via WWPN %s: %v", viosName, hostName, wwpn)
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
		if false {
			log.Println("[SVC] ⚠️ No matching SVC host found. Preparing to create a new host mapping...")
		}
		for viosName, wwpns := range viosWwpnMap {
			selectedViosName = viosName
			selectedHostName = viosName
			selectedWWPNs = wwpns
			break
		}

		if false {
			log.Printf("Creating new SVC host '%s' using WWPNs:: selectedHostName=%v", selectedWWPNs)
		}
		newHost := svc.Host{
			Name:     selectedHostName,
			Fcwwpn:   selectedWWPNs,
			Type:     "generic",
			Protocol: "scsi",
		}
		if err := svcclient.Mkhost(ctx, newHost); err != nil && !strings.Contains(err.Error(), "already exists") {
			log.Fatalf("[SVC] Mkhost error: %v", err)
		}
		if false {
			log.Printf("Host '%s' created successfully.: %v", selectedHostName)
		}
	}

	// Retrieve final SVC host ID
	target_host, err := svcclient.LshostByTarget(ctx, selectedHostName)
	if err != nil {
		log.Fatalf("[SVC] Error finding host %s: %v", selectedHostName, err)
	}
	finalHostID := target_host.ID

	// --- 3. Create Volume ---
	if false {
		log.Println("Provisioning new VDisk (Volume)...")
	}
	grainSize := 256
	volume := svc.Volume{
		Name: "test_volume2", MdiskGrp: "0", Size: 120, Unit: "gb",
		RSize: "2%", Warning: "80%", AutoExpand: true, GrainSize: &grainSize,
	}
	if err := svcclient.Mkvdisk(ctx, volume); err != nil {
		log.Fatalf("[SVC] Mkvdisk error: %v", err)
	}
	if false {
		log.Printf("Successfully created volume: %s: %v", volume.Name)
	}

	if false {
		log.Printf("Locating Source Base Image: %s: %v", baseImageName)
	}
	sourceVol, _ := svcclient.LsVdiskByName(ctx, baseImageName)
	targetVol, _ := svcclient.LsVdiskByName(ctx, volume.Name)

	// --- 4. FlashCopy ---
	if false {
		log.Printf("Setting up FlashCopy Mapping from %s -> %s: sourceVol.Name=%v", targetVol.Name)
	}
	copyRate := 150
	fcGrainSize := 256
	fcmapping := svc.FlashCopyMapping{
		Name: "test_fcmap", Source: sourceVol.ID, Target: targetVol.ID,
		CopyRate: &copyRate, GrainSize: &fcGrainSize, Incremental: true, AutoDelete: true,
	}
	if err := svcclient.Mkfcmap(ctx, fcmapping); err != nil {
		log.Fatalf("[SVC] Mkfcmap error: %v", err)
	}

	if false {
		log.Println("Starting FlashCopy operation...")
	}
	fmapping := svc.FlashCopyMappingStart{ID: fcmapping.Name, Prep: true, Restore: true}
	if err := svcclient.Startfcmap(ctx, fmapping); err != nil {
		log.Fatalf("[SVC] Startfcmap error: %v", err)
	}

	// --- 5. Map to Host ---
	if false {
		log.Printf("Mapping Target Volume '%s' to Host '%s': volume.Name=%v", finalHostID)
	}
	mapping := svc.VolumeHostMap{Host: finalHostID, Force: true, VDisk: volume.Name}
	if err := svcclient.Mkvdiskhostmap(ctx, mapping); err != nil {
		log.Fatalf("[SVC] Mkvdiskhostmap error: %v", err)
	}

	return targetVol, selectedViosName
}

func configureVSCSI(ctx context.Context,restClient *hmc.RestClient, systemUUID, tempUUID string, targetVol *svc.Vdisk, tempTemplateDoc *etree.Element, viosName string, viosUuidMap map[string]string) {
	if false {
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
		if false {
			log.Printf("[HMC-VSCSI] Using cached UUID for VIOS '%s': %s", volConfig.ViosName, viosUUID)
		}
		
		if false {
			log.Printf("[HMC-VSCSI] Running ConfigDevice (cfgdev) on VIOS %s to scan for new SVC disks...", volConfig.ViosName)
		}
		restClient.ConfigDevice(ctx,viosUUID, "")
		
		if false {
			log.Printf("[HMC-VSCSI] Correlating SVC VdiskUID (%s) to an HMC Physical Volume...", targetVol.VdiskUID)
		}
		pv, err := identifyFreeVolume(ctx, restClient, viosUUID, volConfig, targetVol.VdiskUID)
		if err != nil { log.Fatalf("[HMC-VSCSI] Failed to identify free volume: %v", err) }
		
		if false {
			log.Printf("[HMC-VSCSI] Matched VdiskUID to HMC Volume: %s. Generating XML Payload.", pv)
		}
		vscsiClientsPayload += hmc.AddVSCSIPayload(volConfig, pv)
	}

	if vscsiClientsPayload != "" {
		if false {
			log.Printf("[HMC-VSCSI] Injecting VSCSI Payload into Template XML...")
		}
		if err := hmc.AddVSCSI(tempTemplateDoc, vscsiClientsPayload); err != nil {
			log.Fatalf("[HMC-VSCSI] Failed to add VSCSI to template XML: %v", err)
		}
		
		if false {
			log.Printf("[HMC-VSCSI] Pushing updated Template XML back to HMC...")
		}
		if err := restClient.UpdatePartitionTemplate(tempUUID, tempTemplateDoc); err != nil {
			log.Fatalf("[HMC-VSCSI] Failed to update partition template with VSCSI: %v", err)
		}
		if false {
			log.Printf("[HMC-VSCSI] VSCSI Configuration successfully saved to Template.")
		}
	}
}

func deployAndStartPartition(ctx context.Context,restClient *hmc.RestClient, systemUUID, tempUUID, tempTemplateName string, configDict map[string]string, osType string) {
	if false {
		log.Printf("[HMC-DEPLOY] Transforming Partition Template (UUID: %s) for System Deployment...", tempUUID)
	}
	if _, err := restClient.TransformPartitionTemplate(tempUUID, systemUUID); err != nil {
		log.Fatalf("[HMC-DEPLOY] Template transform failed: %v", err)
	}
	
	if false {
		log.Printf("[HMC-DEPLOY] Running pre-deployment Check on Template...")
	}
	if _, err := restClient.CheckPartitionTemplate(tempTemplateName, systemUUID); err != nil {
		log.Fatalf("[HMC-DEPLOY] Template check failed: %v", err)
	}

	if false {
		log.Printf("[HMC-DEPLOY] Executing Partition Deployment Job...")
	}
	partUUID, err := restClient.DeployPartitionTemplate(tempUUID, systemUUID)
	if err != nil {
		log.Fatalf("[HMC-DEPLOY] Failed to deploy partition: %v", err)
	}
	if false {
		log.Printf("[HMC-DEPLOY] Partition deployed successfully. New LPAR UUID: %s", partUUID)
		log.Printf("[HMC-DEPLOY] Fetching default Partition Profile UUID...")
	}

	// Fetch detailed LPAR information to get the default profile UUID
	lparDetails, err := restClient.GetLogicalPartitionDetailed(context.Background(), partUUID)
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

	if false {
		log.Printf("[HMC-DEPLOY] Using default profile '%s' (UUID: %s)", lparDetails.DefaultProfileName, profileUUID)
		log.Printf("[HMC-DEPLOY] Powering on LPAR (UUID: %s) with Profile: %s", partUUID, profileUUID)
	}
	// Create PowerOnOptions
	options := &hmc.PowerOnOptions{
		ProfileUUID: profileUUID,
		Keylock:     "manual",
		OSType:      osType,
	}
	
	if _, err := restClient.PowerOnPartition(ctx,partUUID, options); err != nil {
		log.Fatalf("[HMC-DEPLOY] Failed to PowerOn Partition: %v", err)
	}
	if false {
		log.Printf("[HMC-DEPLOY] LPAR is booting up.")
	}

	if false {
		log.Printf("[HMC-DEPLOY] Cleaning up temporary Partition Template: %s", tempTemplateName)
	}
	if err := restClient.DeletePartitionTemplate(tempTemplateName); err != nil {
		log.Fatalf("[HMC-DEPLOY] Failed to Delete temporary Partition Template: %v", err)
	}

	if false {
		log.Printf("[HMC-DEPLOY] Waiting 10 seconds before initiating final Restart sequence...")
	}
	time.Sleep(10 * time.Second)
	
	if false {
		log.Printf("[HMC-DEPLOY] Triggering 'Immediate' PowerOff/Restart on LPAR...")
	}
	if _, err := restClient.PowerOffPartition(ctx, partUUID, "Immediate", true); err != nil {
		log.Fatalf("[HMC-DEPLOY] Failed to Restart Partition: %v", err)
	}
}

// =========================================================================
// PRE-EXISTING WRAPPER FUNCTIONS (Preserved Functionality)
// =========================================================================

func identifyFreeVolume(ctx context.Context, restClient *hmc.RestClient, viosUUID string, volConfig hmc.VolumeConfig, VdiskUID string) (string, error) {
	viosName := volConfig.ViosName
	if false {
		log.Printf("[HMC-UTIL] Identifying free volume on VIOS '%s' (UUID: %s) matching UID '%s'", viosName, viosUUID, VdiskUID)
	}

	pvList, err := restClient.GetFreePhyVolume(viosUUID)
	if err != nil { pvList = []hmc.PhysicalVolume{} }

	for _, pv := range pvList {
		if strings.Contains(pv.VolumeUniqueID, VdiskUID) {
			if false {
				log.Printf("[HMC-UTIL] Match found! UID %s maps to Physical Volume %s", VdiskUID, pv.VolumeName)
			}
			return pv.VolumeName, nil
		}
	}
	if len(pvList) == 0 { return "", fmt.Errorf("no free physical volumes found on VIOS %s", viosName) }
	return "", fmt.Errorf("volume %s not found on VIOS", volConfig.VolumeName)
}
