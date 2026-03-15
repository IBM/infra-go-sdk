package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
	hmc "github.com/sudeeshjohn/PowerHMC"
	"github.com/sudeeshjohn/svc-go-sdk/svc"
)

// VolumeConfig defines the configuration for a volume
type VolumeConfig struct {
	ViosName   string // Name of the VIOS managing the volume
	VolumeName string // Name of the volume (e.g., hdisk1)
}

// PartitionConfig holds the high-level configuration for the deployment
type PartitionConfig struct {
	OSType string
}

func main() {
	// --- Command-line flags with hardcoded defaults ---
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("username", "REDACTED_HMC_USER<==", "Username")
	password := flag.String("password", "REDACTED_HMC_PASS<==", "Password")
	verbose := flag.Bool("verbose", true, "Enable verbose output")
	osType := flag.String("os-type", "linux", "OS type (aix, linux, aix_linux, ibmi)")
	listTemplate := flag.Bool("list-template", true, "List all partition template IDs")
	templateName := flag.String("template-name", "", "Get AtomID for a specific partition template name")
	systemName := flag.String("system-name", "LTC09U31-ZZ", "Managed system name")
	flag.Parse()

	// --- Initialize & Authenticate HMC ---
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if *verbose {
		log.Printf("Attempting to log on to HMC at %s with username %s", *hmcIP, *username)
	}
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer func() {
		if err := restClient.Logoff(); err != nil {
			log.Printf("Logoff failed: %v", err)
		} else if *verbose {
			log.Println("Logged off successfully")
		}
	}()

	// --- Handle Template Listing/Querying ---
	if *listTemplate {
		handleListTemplates(restClient, *verbose)
	}
	if *templateName != "" {
		handleGetTemplateID(restClient, *templateName, *verbose)
	}

	// --- Main Partition Creation Workflow ---
	if *systemName != "" && *osType != "" {
		systemUUID := resolveSystemUUID(restClient, *systemName, *verbose)
		configDict := buildLparConfigDict()

		config := PartitionConfig{
			OSType: *osType,
		}

		// 1. Check if LPAR already exists
		ensureLparDoesNotExist(restClient, systemUUID, configDict["vm_name"], *verbose)

		// 2. Prepare the Partition Template
		tempUUID, tempTemplateName, tempTemplateDoc, err := prepareLparTemplate(restClient, config, configDict, *verbose)
		if err != nil {
			log.Fatalf("Template preparation failed: %v", err)
		}

		// 3. Provision SVC Storage
		targetVol := provisionSVCStorage()

		// 4. Configure VSCSI and update the template
		configureVSCSI(restClient, systemUUID, tempUUID, targetVol, tempTemplateDoc, *verbose)

		// 5. Deploy, Start, and Cleanup
		deployAndStartPartition(restClient, systemUUID, tempUUID, tempTemplateName, configDict, *osType, *verbose)
		
		// 6. Run end-of-workflow verifications
		runVerificationTests(restClient, systemUUID, *systemName, *verbose)
	}
}

// =========================================================================
// WORKFLOW HELPER FUNCTIONS
// =========================================================================

func handleListTemplates(restClient *hmc.HmcRestClient, verbose bool) {
	if verbose { log.Printf("Listing all partition template IDs") }
	ids, err := restClient.ListPartitionTemplateIDs(verbose)
	if err != nil {
		log.Fatalf("Failed to list partition template IDs: %v", err)
	}
	fmt.Println("Partition Template IDs:")
	for i, id := range ids {
		fmt.Printf("%d: %s\n", i+1, id)
	}
}

func handleGetTemplateID(restClient *hmc.HmcRestClient, templateName string, verbose bool) {
	if verbose { log.Printf("Retrieving AtomID for template name: %s", templateName) }
	id, err := restClient.GetPartitionTemplateID(templateName, verbose)
	if err != nil {
		log.Fatalf("Failed to get template ID for %s: %v", templateName, err)
	}
	fmt.Printf("Template ID for %s: %s\n", templateName, id)
}

func resolveSystemUUID(restClient *hmc.HmcRestClient, systemName string, verbose bool) string {
	uuid, systemElem, err := restClient.GetManagedSystemByName(systemName, verbose)
	if err != nil {
		log.Fatalf("Failed to get managed system: %v", err)
	}
	if uuid == "" {
		log.Fatalf("Given system '%s' is not present", systemName)
	}
	if verbose {
		fmt.Printf("Managed System UUID: %s\n", uuid)
		maxLpars := systemElem.FindElement("//MaximumPartitions")
		if maxLpars != nil {
			fmt.Printf("Maximum Partitions for system %s: %s\n", uuid, maxLpars.Text())
		}
	}
	return uuid
}

func buildLparConfigDict() map[string]string {
	// Centralized hardcoded LPAR defaults
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
	configDict["weight"] = "128" // Assuming uncapped
	configDict["proc_comp_mode"] = "Default"
	configDict["shared_proc_pool"] = "0"
	return configDict
}

func ensureLparDoesNotExist(restClient *hmc.HmcRestClient, systemUUID, vmName string, verbose bool) {
	if verbose { log.Printf("Checking for existing LPAR with name %s", vmName) }
	existingUUID, _, err := restClient.GetLogicalPartition(systemUUID, vmName, "", verbose)
	if err != nil {
		log.Fatalf("Failed to check for existing LPAR: %v", err)
	}
	if existingUUID != "" {
		log.Fatalf("LPAR with name %s already exists with UUID %s", vmName, existingUUID)
	}
}

func prepareLparTemplate(restClient *hmc.HmcRestClient, config PartitionConfig, configDict map[string]string, verbose bool) (string, string, *etree.Element, error) {
	var referenceTemplate string
	
	switch config.OSType {
	case "aix", "linux", "aix_linux":
		referenceTemplate = "QuickStart_lpar_rpa_2"
	case "ibmi":
		referenceTemplate = "QuickStart_lpar_IBMi_2"
	default:
		return "", "", nil, fmt.Errorf("invalid os-type: %s (must be aix, linux, aix_linux, or ibmi)", config.OSType)
	}

	// Generate a unique temporary template name
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	tempTemplateName := fmt.Sprintf("hmctool_powervm_create_%04d", rng.Intn(9000)+1000)

	// Copy template
	if err := restClient.CopyPartitionTemplate(referenceTemplate, tempTemplateName, verbose); err != nil {
		return "", "", nil, fmt.Errorf("failed to copy template from %s to %s: %v", referenceTemplate, tempTemplateName, err)
	}
	fmt.Printf("Successfully copied template to %s\n", tempTemplateName)

	// Retrieve XML
	tempTemplateDoc, err := restClient.GetPartitionTemplate("", tempTemplateName, verbose)
	if err != nil || tempTemplateDoc == nil {
		return "", "", nil, fmt.Errorf("failed to retrieve temporary template %s: %v", tempTemplateName, err)
	}
	
	atomIDs := tempTemplateDoc.FindElements("//AtomID")
	if len(atomIDs) == 0 { 
		return "", "", nil, fmt.Errorf("AtomID not found for temporary template") 
	}
	tempUUID := atomIDs[0].Text()

	// Update XML Settings
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

	return tempUUID, tempTemplateName, tempTemplateDoc, nil
}

func provisionSVCStorage() *svc.Vdisk {
	// Centralized SVC Configs
	svcIP, svcUser, svcPass := "192.0.2.8", "REDACTED_SVC_USER<==", "REDACTED_HMC_PASS<=="
	baseImageName := "image-ibm-default-centos-10"

	svcclient := svc.NewClient(svcIP, svcUser, svcPass).WithTLSInsecure()
	if err := svcclient.Authenticate(); err != nil {
		log.Fatalf("SVC auth error: %v", err)
	}
	fmt.Println("✅ Authenticated to SVC")

	host := svc.Host{
		Name:     "host2",
		Fcwwpn:   []string{"10000090FADC7453", "10000090FADC7454"},
		Type:     "generic",
		Protocol: "scsi",
	}

	// Host logic
	for _, wwpn := range host.Fcwwpn {
		existingHost, err := svcclient.GetHostByWWPN(wwpn)
		if err == nil {
			fmt.Printf("WWPN %s is already associated with host: %s\n", wwpn, existingHost)
			host.Name = existingHost
		} else if !strings.Contains(err.Error(), "not found") {
			err = svcclient.Mkhost(svc.Host{Name: "host1", Fcwwpn: []string{"100000620B42EB09", "100000620B42EB0A"}, Type: "generic", Protocol: "scsi"})
			if err != nil && !strings.Contains(err.Error(), "already exists") {
				log.Fatalf("Mkhost error: %v", err)
			}
		}
	}

	target_host, err := svcclient.LshostByTarget(host.Name)
	if err != nil {
		log.Fatalf("Error finding host: %v", err)
	}
	host.Name = target_host.ID

	// Create Volume
	volume := svc.Volume{
		Name: "test_volume2", MdiskGrp: "0", Size: 120, Unit: "gb", 
		RSize: "2%", Warning: "80%", AutoExpand: true, GrainSize: 256,
	}
	if err := svcclient.Mkvdisk(volume); err != nil {
		log.Fatalf("Mkvdisk error: %v", err)
	}
	fmt.Printf("Successfully created disk: %s\n", volume.Name)

	sourceVol, _ := svcclient.LsVdiskByName(baseImageName)
	targetVol, _ := svcclient.LsVdiskByName(volume.Name)

	// FlashCopy
	fcmapping := svc.FlashCopyMapping{
		Name: "test_fcmap", Source: sourceVol.ID, Target: targetVol.ID, 
		CopyRate: 150, GrainSize: 256, Incremental: true, AutoDelete: true,
	}
	if err := svcclient.Mkfcmap(fcmapping); err != nil {
		log.Fatalf("Mkfcmap error: %v", err)
	}

	fmapping := svc.FlashCopyMappingStart{ID: fcmapping.Name, Prep: true, Restore: true}
	if err := svcclient.Startfcmap(fmapping); err != nil {
		log.Fatalf("Startfcmap error: %v", err)
	}

	// Map to Host
	mapping := svc.VolumeHostMap{Host: host.Name, Force: true, VDisk: "test_volume2"}
	if err := svcclient.Mkvdiskhostmap(mapping); err != nil {
		log.Fatalf("Mkvdiskhostmap error: %v", err)
	}

	return targetVol
}

func configureVSCSI(restClient *hmc.HmcRestClient, systemUUID, tempUUID string, targetVol *svc.Vdisk, tempTemplateDoc *etree.Element, verbose bool) {
	volumeConfigs := []hmc.VolumeConfig{
		{ViosName: "ltc09u31-vios1", VolumeName: targetVol.Name},
	}

	vscsiClientsPayload := ""
	for _, volConfig := range volumeConfigs {
		viosUUID, err := hmc.GetViosID(restClient, systemUUID, volConfig.ViosName, verbose)
		if err != nil { log.Fatalf("Failed to get VIOS ID: %v", err) }
		
		restClient.ConfigDevice(viosUUID, "", verbose)
		
		pv, err := identifyFreeVolume(restClient, systemUUID, volConfig, targetVol.VdiskUID, verbose)
		if err != nil { log.Fatalf("Failed to identify free volume: %v", err) }
		
		vscsiClientsPayload += hmc.AddVSCSIPayload(volConfig, pv, verbose)
	}

	if vscsiClientsPayload != "" {
		if err := hmc.AddVSCSI(tempTemplateDoc, vscsiClientsPayload); err != nil {
			log.Fatalf("Failed to add VSCSI to template XML: %v", err)
		}
		if err := restClient.UpdatePartitionTemplate(tempUUID, tempTemplateDoc, verbose); err != nil {
			log.Fatalf("Failed to update partition template with VSCSI: %v", err)
		}
	}
}

func deployAndStartPartition(restClient *hmc.HmcRestClient, systemUUID, tempUUID, tempTemplateName string, configDict map[string]string, osType string, verbose bool) {
	// Transform & Check Template
	if err := TransormTemp(restClient, tempUUID, systemUUID, verbose); err != nil {
		log.Fatalf("Template transform failed: %v", err)
	}
	if err := CheckTemp(restClient, tempTemplateName, systemUUID, verbose); err != nil {
		log.Fatalf("Template check failed: %v", err)
	}

	// Deploy Partition
	partUUID, err := restClient.DeployPartitionTemplate(tempUUID, systemUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to deploy partition: %v", err)
	}
	fmt.Printf("Partition deployed successfully. UUID: %s\n", partUUID)

	profileUUID, _ := restClient.GetPartitionProfile(partUUID, verbose)

	// Power On
	PartitionPowerOn(restClient, systemUUID, partUUID, profileUUID, "manual", "", osType, verbose)

	// Cleanup Temp Template
	DeleteTempPartitionTemplate(restClient, tempTemplateName, verbose)

	// Sleep and Restart
	time.Sleep(10 * time.Second)
	PowerOff(restClient, systemUUID, partUUID, "Immediate", true, verbose)
}

func runVerificationTests(restClient *hmc.HmcRestClient, systemUUID, systemName string, verbose bool) {
	_ = GetMagdSystemsByName(restClient, systemName, verbose)
	_ = GetMagdSystemsQuick(restClient, verbose)
	_ = GetMagdSystemQuick(restClient, systemUUID, verbose)
	_ = GetLgclPartitions(restClient, systemUUID, verbose)
}

// =========================================================================
// PRE-EXISTING WRAPPER FUNCTIONS (Preserved Functionality)
// =========================================================================

func GetLgclPartitionQuick(restClient *hmc.HmcRestClient, partUUID string, verbose bool) error {
	_, err := restClient.GetLogicalPartitionQuick(partUUID, verbose)
	if err != nil { log.Fatalf("Failed to get Logical partition Quick: %v", err) }
	return err
}
func GetLgclPartitions(restClient *hmc.HmcRestClient, systemUUID string, verbose bool) error {
	_, err := restClient.GetLogicalPartitions(systemUUID, verbose)
	if err != nil { log.Fatalf("Failed to get Logical partitions: %v", err) }
	return err
}
func GetMagdSystemQuick(restClient *hmc.HmcRestClient, systemUUID string, verbose bool) error {
	_, err := restClient.GetManagedSystemQuick(systemUUID, verbose)
	if err != nil { log.Fatalf("Failed to get Manged Systems Quick: %v", err) }
	return err
}
func GetMagdSystemsQuick(restClient *hmc.HmcRestClient, verbose bool) error {
	_, err := restClient.GetManagedSystemsQuick(verbose)
	if err != nil { log.Fatalf("Failed to get Manged Systems: %v", err) }
	return err
}
func GetMagdSystemsByName(restClient *hmc.HmcRestClient, systemName string, verbose bool) error {
	_, _, err := restClient.GetManagedSystemByName(systemName, verbose)
	if err != nil { log.Fatalf("Failed to get Managed System by name: %v", err) }
	return err
}
func TransormTemp(restClient *hmc.HmcRestClient, tempUUID, systemUUID string, verbose bool) error {
	_, err := restClient.TransformPartitionTemplate(tempUUID, systemUUID, verbose)
	return err
}
func CheckTemp(restClient *hmc.HmcRestClient, tempTemplateName, systemUUID string, verbose bool) error {
	_, err := restClient.CheckPartitionTemplate(tempTemplateName, systemUUID, verbose)
	return err
}
func PartitionPowerOn(restClient *hmc.HmcRestClient, systemUUID, lparUUID, profileUUID, keylock, iIPLsource, osType string, verbose bool) (string, error) {
	_, err := restClient.PowerOnPartition(systemUUID, lparUUID, profileUUID, "manual", "", osType, verbose)
	if err != nil { log.Fatalf("Failed to PowerOn Partition: %v", err) }
	log.Printf("Rebooted successfully")
	return "", nil
}
func PowerOff(restClient *hmc.HmcRestClient, systemUUID, lparUUID, shutdownOption string, restart bool, verbose bool) (*etree.Document, error) {
	doc, err := restClient.PowerOffPartition(systemUUID, lparUUID, shutdownOption, restart, verbose)
	if err != nil { log.Fatalf("Failed to Restart Partition: %v", err) }
	return doc, err
}
func DeleteTempPartitionTemplate(restClient *hmc.HmcRestClient, templateName string, verbose bool) {
	if err := restClient.DeletePartitionTemplate(templateName, verbose); err != nil {
		log.Fatalf("Failed to Delete Partition Template: %v", err)
	}
}
func getMacAdddress(restClient *hmc.HmcRestClient, systemUUID string, partUUID string, verbose bool) (string, error) {
	clientNetAdapter, err := restClient.GetClientNetworkAdapter(systemUUID, partUUID, verbose)
	if err != nil { log.Fatalf("Failed to retrieve client network adapter: %v", err) }
	return clientNetAdapter.FindElement("//MACAddress").Text(), nil
}
func identifyFreeVolume(restClient *hmc.HmcRestClient, systemUUID string, volConfig hmc.VolumeConfig, VdiskUID string, verbose bool) (string, error) {
	viosName := volConfig.ViosName
	viosUUID, err := hmc.GetViosID(restClient, systemUUID, viosName, verbose)
	if err != nil { return "", err }

	pvList, err := restClient.GetFreePhyVolume(viosUUID, verbose)
	if err != nil { pvList = []hmc.PhysicalVolume{} }

	for _, pv := range pvList {
		if strings.Contains(pv.VolumeUniqueID, VdiskUID) {
			return pv.VolumeName, nil
		}
	}
	if len(pvList) == 0 { return "", fmt.Errorf("no free physical volumes found on VIOS %s", viosName) }
	return "", fmt.Errorf("volume %s not found", volConfig.VolumeName)
}