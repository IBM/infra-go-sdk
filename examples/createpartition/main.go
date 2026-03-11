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

func main() {
	// Command-line flags
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("username", "", "Username")
	password := flag.String("password", "", "Password")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	osType := flag.String("os-type", "", "OS type (aix, linux, aix_linux, ibmi)")
	listTemplate := flag.Bool("list-template", false, "List all partition template IDs")
	templateName := flag.String("template-name", "", "Get AtomID for a specific partition template name")
	systemName := flag.String("system-name", "", "Managed system name")
	flag.Parse()

	// Initialize HmcRestClient
	restClient := hmc.NewHmcRestClient(*hmcIP)

	// Logon
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
	// Hardcode virt_network_config for now
	virtNetworkConfigs := []hmc.VirtualNetworkConfig{{NetworkName: "VNET0", SlotNumber: 49, VirtualSlotNumber: 49}}

	// Handle managed system operations if system-name is provided
	var systemUUID string
	configDict := make(map[string]string) // Initialize configDict
	if *systemName != "" {
		uuid, systemElem, err := restClient.GetManagedSystem(*systemName, *verbose)
		if err != nil {
			log.Fatalf("Failed to get managed system: %v", err)
		}
		if uuid == "" {
			log.Fatalf("Given system '%s' is not present", *systemName)
		}
		systemUUID = uuid
		if *verbose {
			fmt.Printf("Managed System UUID: %s\n", systemUUID)
		}
		// Fetch MaximumPartitions for the system
		if *verbose {
			log.Printf("Fetching MaximumPartitions for system UUID: %s", systemUUID)
		}
		maxLpars := systemElem.FindElement("//MaximumPartitions")
		if maxLpars == nil {
			log.Fatalf("Failed to fetch MaximumPartitions for system %s: %v", systemUUID, err)
		}
		fmt.Printf("Maximum Partitions for system %s: %s\n", systemUUID, maxLpars.Text())

		// Hardcoded values as per your request
		vmName := "test-test"
		proc := 2
		procUnit := 2
		mem := 2048
		maxVirtualSlots := 50
		procMode := "uncapped"             // Assumption based on weight logic
		weight10 := 128                    // Hardcoded assumption
		procCompatibilityMode := "Default" // Hardcoded assumption
		sharedProcPool := "0"              // Hardcoded default

		// Populate configDict with hardcoded values
		configDict["vm_name"] = vmName
		configDict["proc"] = strconv.Itoa(proc)
		configDict["max_proc"] = strconv.Itoa(proc)          // Default to proc value
		configDict["min_proc"] = "1"                         // Reasonable default
		configDict["proc_unit"] = strconv.Itoa(procUnit)     // Convert to string
		configDict["max_proc_unit"] = strconv.Itoa(procUnit) // Default to proc_unit
		configDict["min_proc_unit"] = "1"                    // Reasonable default
		configDict["mem"] = strconv.Itoa(mem)
		configDict["max_mem"] = strconv.Itoa(mem) // Default to mem value
		configDict["min_mem"] = "1024"            // Reasonable default
		configDict["max_virtual_slots"] = strconv.Itoa(maxVirtualSlots)
		configDict["proc_mode"] = procMode
		if procMode == "uncapped" {
			configDict["weight"] = strconv.Itoa(weight10)
		} else {
			configDict["weight"] = "0"
		}
		configDict["proc_comp_mode"] = procCompatibilityMode
		configDict["shared_proc_pool"] = sharedProcPool

		// Log configDict if verbose
		if *verbose {
			log.Printf("Configuration dictionary: %+v", configDict)
		}

		// Check proc_comp_mode
		if procCompMode, ok := configDict["proc_comp_mode"]; ok && procCompMode != "" {
			suppCompatModes := systemElem.FindElements("//SupportedPartitionProcessorCompatibilityModes")
			supportedModes := make([]string, 0, len(suppCompatModes))
			for _, modeElem := range suppCompatModes {
				text := modeElem.Text()
				if text == "default" {
					supportedModes = append(supportedModes, "Default")
				} else {
					processed := strings.ReplaceAll(text, "Plus", "plus")
					supportedModes = append(supportedModes, processed)
				}
			}
			found := false
			for _, mode := range supportedModes {
				if mode == procCompMode {
					found = true
					break
				}
			}
			if !found {
				log.Fatalf("unsupported proc_compat_mode: %s, Supported proc_compat_modes are %v", procCompMode, supportedModes)
			}
		}
	}

	// List all partition template IDs if --list-template is set
	if *listTemplate {
		if *verbose {
			log.Printf("Listing all partition template IDs")
		}
		ids, err := restClient.ListPartitionTemplateIDs(*verbose)
		if err != nil {
			log.Fatalf("Failed to list partition template IDs: %v", err)
		}
		fmt.Println("Partition Template IDs:")
		for i, id := range ids {
			fmt.Printf("%d: %s\n", i+1, id)
			if *verbose {
				log.Printf("Template ID %d: %s", i+1, id)
			}
		}
	}

	// Get specific template ID by name if --template-name is set
	if *templateName != "" {
		if *verbose {
			log.Printf("Retrieving AtomID for template name: %s", *templateName)
		}
		id, err := restClient.GetPartitionTemplateID(*templateName, *verbose)
		if err != nil {
			log.Fatalf("Failed to get template ID for %s: %v", *templateName, err)
		}
		fmt.Printf("Template ID for %s: %s\n", *templateName, id)
	}

	// Perform template copy and partition creation if os-type is set
	if *osType != "" {
		if *verbose {
			log.Printf("Checking for existing LPAR with name %s on system UUID %s", configDict["vm_name"], systemUUID)
		}
		existingUUID, _, err := restClient.GetLogicalPartition(systemUUID, configDict["vm_name"], "", *verbose)
		if err != nil {
			log.Fatalf("Failed to check for existing LPAR: %v", err)
		}
		if existingUUID != "" {
			log.Fatalf("LPAR with name %s already exists with UUID %s", configDict["vm_name"], existingUUID)
		}
		if *verbose {
			log.Printf("No existing LPAR found with name %s", configDict["vm_name"])
		}
		var referenceTemplate string
		switch *osType {
		case "aix", "linux", "aix_linux":
			referenceTemplate = "QuickStart_lpar_rpa_2"
		case "ibmi":
			referenceTemplate = "QuickStart_lpar_IBMi_2"
		default:
			log.Fatalf("Invalid os-type: %s. Must be aix, linux, aix_linux, or ibmi", *osType)
		}

		// Generate a unique temporary template name
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		tempTemplateName := fmt.Sprintf("hmctool_powervm_create_%04d", rng.Intn(9000)+1000)
		if *verbose {
			log.Printf("Generated temporary template name: %s", tempTemplateName)
		}

		// Copy the template
		if *verbose {
			log.Printf("Copying template from %s to %s", referenceTemplate, tempTemplateName)
		}
		err = restClient.CopyPartitionTemplate(referenceTemplate, tempTemplateName, *verbose)
		if err != nil {
			log.Fatalf("Failed to copy template from %s to %s: %v", referenceTemplate, tempTemplateName, err)
		}
		fmt.Printf("Successfully copied template from %s to %s\n", referenceTemplate, tempTemplateName)

		// Retrieve the copied template's XML
		if *verbose {
			log.Printf("Retrieving AtomID for temporary template: %s", tempTemplateName)
		}
		tempTemplateDoc, err := restClient.GetPartitionTemplate("", tempTemplateName, *verbose)
		if err != nil || tempTemplateDoc == nil {
			log.Fatalf("Failed to retrieve temporary template %s: %v", tempTemplateName, err)
		}
		atomIDs := tempTemplateDoc.FindElements("//AtomID")
		if len(atomIDs) == 0 {
			log.Fatalf("AtomID not found for temporary template %s", tempTemplateName)
		}
		tempUUID := atomIDs[0].Text()
		fmt.Printf("Temporary template UUID: %s\n", tempUUID)

		// Update the temporary template XML with configDict values
		if *verbose {
			log.Printf("Updating temporary template XML with configDict")
		}
		err = restClient.UpdateLparNameAndIDToDom(tempTemplateDoc, configDict)
		if err != nil {
			log.Fatalf("Failed to update temporary template XML: %v", err)
		}

		// Update processor and memory settings in the XML
		if *verbose {
			log.Printf("Updating processor and memory settings in temporary template XML")
		}
		err = restClient.UpdateProcMemSettingsToDom(tempTemplateDoc, configDict)
		if err != nil {
			log.Fatalf("Failed to update processor and memory settings: %v", err)
		}

		// Update virtual network settings in the XML
		if *verbose {
			log.Printf("Updating virtual network settings in temporary template XML")
		}
		err = restClient.UpdateVirtualNWSettingsToDom(tempTemplateDoc, virtNetworkConfigs)
		if err != nil {
			log.Fatalf("Failed to update virtual network settings: %v", err)
		}

		// Print the updated XML for verification
		if *verbose {
			doc := etree.NewDocument()
			doc.SetRoot(tempTemplateDoc)
			xmlString, err := doc.WriteToString()
			if err != nil {
				log.Printf("Failed to serialize updated XML: %v", err)
			} else {
				log.Printf("Updated XML before VSCSI:\n%s", xmlString)
			}
		}
		// Creating host in svc
		svcclient := svc.NewClient("192.0.2.8", "REDACTED_SVC_USER<==", "REDACTED_HMC_PASS<==").WithTLSInsecure()

		if err := svcclient.Authenticate(); err != nil {
			log.Fatalf("auth error: %v", err)
		}
		fmt.Println("✅ Authenticated")

		systemInfo, err := svcclient.Lssystem()
		if err != nil {
			log.Fatalf("lssystem error: %v", err)
		}
		fmt.Printf("System: %+v\n", systemInfo)

		// Define the host parameters
		host := svc.Host{
			Name:     "host2",
			Fcwwpn:   []string{"100000620B42EB0A", "100000620B42EB09"},
			Type:     "generic",
			Protocol: "scsi",
		}

		// Check if any WWPN is already associated with a host
		for _, wwpn := range host.Fcwwpn {
			existingHost, err := svcclient.GetHostByWWPN(wwpn)
			if err == nil {
				fmt.Printf("WWPN %s is already associated with host: %s. Skipping creation.\n", wwpn, existingHost)
				host.Name = existingHost
			} else if !strings.Contains(err.Error(), "not found") {
				err = svcclient.Mkhost(svc.Host{Name: "host1", Fcwwpn: []string{"100000620B42EB09", "100000620B42EB0A"}, Type: "generic", Protocol: "scsi"})
				if err != nil {
					if strings.Contains(err.Error(), "CMMVC6035E") || strings.Contains(err.Error(), "object already exists") {
						fmt.Println("Host already exists, skipping creation.")
					} else {
						log.Fatalf("Mkhost error: %v", err)
					}
				} else {
					fmt.Println("Successfully created host.")
				}
			}
		}

		targetHost := host.Name
    	fmt.Printf("Searching for host: %s...\n", targetHost)

    	target_host, err := svcclient.LshostByTarget(targetHost)
    	if err != nil {
        if strings.Contains(err.Error(), "CMMVC5754E") {
            fmt.Printf("❌ Host '%s' not found (CMMVC5754E)\n", targetHost)
        } else {
            log.Fatalf("Error: %v", err)
        }
		} else {
			// 'host' is now a direct pointer to the object
			fmt.Printf("✅ Found Host: %s (ID: %s)\n", target_host.Name, target_host.ID)
			fmt.Printf("   Status: %s, Protocol: %s, Portset: %s\n", target_host.Status, host.Protocol, target_host.PortsetName)
		}
		host.Name = target_host.ID

		//Create Volume

		// Create a Volume instance
		volume := svc.Volume{
			Name:       "test_volume2",
			MdiskGrp:   "0",
			Size:       120,
			Unit:       "gb",
			RSize:      "2%",
			Warning:    "80%",
			AutoExpand: true,
			GrainSize:  256,
		}

		// Create the volume using Mkvdisk
		if err := svcclient.Mkvdisk(volume); err != nil {
			log.Fatalf("Mkvdisk error: %v", err)
		} else {
			fmt.Printf("Successfully created disk with name: %s\n", volume.Name)
		}
		sourceVol, err := svcclient.LsVdisk("image-ibm-default-centos-10")
		if err != nil {
			log.Fatalf("LsVdisk error: %v", err)
		}
		fmt.Printf("SOURCE VOLUEME: %s\n", sourceVol[0].ID)
		targetVol, err := svcclient.LsVdisk(volume.Name)
		if err != nil {
			log.Fatalf("LsVdisk error: %v", err)
		}
		fmt.Printf("TARGET VOLUEME: %s\n", targetVol[0].ID)
		// Create a FlashCopyMapping instance
		fcmapping := svc.FlashCopyMapping{
			Name:        "test_fcmap",
			Source:      sourceVol[0].ID, //Soure volume ID
			Target:      targetVol[0].ID, //Target Volume ID
			CopyRate:    150,             //Specifies the copy rate. The rate value can be 0 - 150;attribute value:141 - 150, Data copied/sec:2 GB, 256 KB grains/sec:8192,64 KB grains/sec:32768
			GrainSize:   256,
			Incremental: true,
			AutoDelete:  true, //Specifies that a mapping is to be deleted when the background copy completes
			//ConsistGrp:  "test_fcgrp",
		}

		// Create the FlashCopy mapping
		if err := svcclient.Mkfcmap(fcmapping); err != nil {
			log.Fatalf("Mkfcmap error: %v", err)
		} else {
			fmt.Printf("Successfully created FlashCopy mapping with name: %s\n", fcmapping.Name)
		}

		mappings, err := svcclient.Lsfcmap(fcmapping.Name)
		if err != nil {
			log.Fatalf("Lsfcmap error: %v", err)
		}
		if len(mappings) == 0 {
			log.Fatalf("No FlashCopy mapping found with name: %s", fcmapping.Name)
		}
		fmt.Printf("Found FlashCopy mapping: %s\n", fcmapping.Name)

		// Create a FlashCopyMappingStart instance
		//id := mappings[0].ID
		//Id, err := strconv.Atoi(id)
		fmapping := svc.FlashCopyMappingStart{
			ID:      fcmapping.Name,
			Prep:    true, // Prepare the mapping before starting
			Restore: true, // Force start if target is in use
		}

		// Start the FlashCopy mapping
		if err := svcclient.Startfcmap(fmapping); err != nil {
			log.Fatalf("Startfcmap error: %v", err)
		} else {
			fmt.Printf("Successfully started FlashCopy mapping with ID: %s\n", mappings[0].Name)
		}

		// MAPPING VOLUME TO HOST

		// Create a VolumeHostMap instance
		mapping := svc.VolumeHostMap{
			Host: host.Name, // Host name or ID
			SCSI: "",       // Optional SCSI LUN ID
			//SCSI: "1",       // Optional SCSI LUN ID

			Force: true,           // Optional force flag
			VDisk: "test_volume2", // Volume name or ID
		}
fmt.Printf("Host information that is going to be mapped: %s\n", mapping.Host)
		// Create the volume to host mapping
		if err := svcclient.Mkvdiskhostmap(mapping); err != nil {
			log.Fatalf("Mkvdiskhostmap error: %v", err)
		} else {
			fmt.Printf("Successfully created volume host mapping for volume: %s to host: %s\n", mapping.VDisk, mapping.Host)

		}

		// Define volume configs, structured like virtNetworkConfigs
		volumeConfigs := []hmc.VolumeConfig{
			{ViosName: "ltc13u29-vios1", VolumeName: targetVol[1].Name},
			// Add more as needed
		}

		// Add VSCSI configuration and update template
		if len(volumeConfigs) > 0 {
			vscsiClientsPayload := ""
			viosUUID := ""
			for _, volConfig := range volumeConfigs {
				viosUUID, err = hmc.GetViosID(restClient, systemUUID, volConfig.ViosName, *verbose)
				fmt.Printf("RETRURNED VIOSUUID: %s\n", viosUUID)
				err := restClient.ConfigDevice(viosUUID, "", *verbose)
				pv, err := identifyFreeVolume(restClient, systemUUID, volConfig, targetVol[1].VdiskUID, *verbose)
				if err != nil {
					log.Fatalf("Failed to identify free volume: %v", err)
				}
				payload := hmc.AddVSCSIPayload(volConfig, pv, *verbose)
				vscsiClientsPayload += payload
			}
			if vscsiClientsPayload != "" {
				if *verbose {
					log.Printf("Adding VSCSI client adapters to template XML")
				}
				err := hmc.AddVSCSI(tempTemplateDoc, vscsiClientsPayload)
				if err != nil {
					log.Fatalf("Failed to add VSCSI to template XML: %v", err)
				}
				// Update the partition template with VSCSI configuration
				if *verbose {
					log.Printf("Updating partition template with UUID: %s after VSCSI configuration", tempUUID)
				}
				if err := restClient.UpdatePartitionTemplate(tempUUID, tempTemplateDoc, *verbose); err != nil {
					log.Fatalf("Failed to update partition template with VSCSI: %v", err)
				}
				fmt.Printf("Successfully updated partition template with VSCSI for UUID: %s\n", tempUUID)
			}
		}

		// Print the final updated XML for verification
		if *verbose {
			doc := etree.NewDocument()
			doc.SetRoot(tempTemplateDoc)
			xmlString, err := doc.WriteToString()
			if err != nil {
				log.Printf("Failed to serialize final updated XML: %v", err)
			} else {
				log.Printf("Final updated XML:\n%s", xmlString)
			}
		}
		// Transforming template
		if *verbose {
			log.Printf("Transforming partition template %s", tempTemplateName)
		}
		err = TransormTemp(restClient, tempUUID, systemUUID, *verbose)
		if err != nil {
			log.Fatalf("Failed to create partition: %v", err)
		}
		// Checking partition template
		if *verbose {
			log.Printf("Checking partition template %s\n", tempUUID)
		}
		fmt.Printf("Template tranformation successful")
		err = CheckTemp(restClient, tempTemplateName, systemUUID, *verbose)
		if err != nil {
			log.Fatalf("Failed to create partition: %v", err)
		}
		fmt.Printf("Template tranformation successful")

		// Create a partition using the updated template
		if *verbose {
			log.Printf("Creating partition for system %s using updated template %s", systemUUID, tempTemplateName)
		}
		partUUID, err := restClient.DeployPartitionTemplate(tempUUID, systemUUID, *verbose)
		if err != nil {
			log.Fatalf("Failed to create partition: %v", err)
		}
		fmt.Printf("Partition creation job ID: %s\n", partUUID)

		// Retrieve partition properties
		partitionProps, err := restClient.GetLogicalPartitionQuick(partUUID, *verbose)
		if err != nil {
			log.Fatalf("Failed to retrieve partition properties: %v", err)
		}

		// Add AssociatedManagedSystem
		//partitionProps["AssociatedManagedSystem"] = *systemName

		clientMacAddress, err := getMacAdddress(restClient, systemUUID, partUUID, *verbose)
		if err != nil {
			log.Fatalf("Failed to retrieve client network adapter: %v", err)
		}
		log.Printf("Mac Address: %s", clientMacAddress)
		//partitionProps["MacAddress"] = clientMacAddress

		profileUUID, err := restClient.GetPartitionProfile(partUUID, *verbose)
		//partitionProps["ProfileUUID"] = profileUUID

		// Powering on the partition created
		_, err = PartitionPowerOn(restClient, systemUUID, partUUID, profileUUID, "manual", "", *osType, *verbose)

		//Deleting the temporary partition template profile created
		DeleteTempPartitionTemplate(restClient, tempTemplateName, *verbose)

		time.Sleep(10000)
		//Restarting partition
		_, _ = PowerOff(restClient, systemUUID, partUUID, "Immediate", true, *verbose)
		// Print partition properties
		if *verbose {
			log.Printf("Partition properties: %+v", partitionProps)
		}

		// Log configDict if verbose
		if *verbose && len(configDict) > 0 {
			log.Printf("Configuration dictionary: %+v", configDict)
		}
		_ = GetMagdSystemsByName(restClient,systemName, *verbose)
		_ = GetMagdSystemsQuick(restClient, *verbose)
		_ = GetMagdSystemQuick(restClient, systemUUID, *verbose)
		_ = GetLgclPartitions(restClient, systemUUID, *verbose)
		_ = GetLgclPartitionQuick(restClient, partUUID, *verbose)
	}

}
func GetLgclPartitionQuick(restClient *hmc.HmcRestClient, partUUID string, verbose bool) error {
	_, err := restClient.GetLogicalPartitionQuick(partUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to get Logical partition Quick, with error %v", err)
	}
	return err
}
func GetLgclPartitions(restClient *hmc.HmcRestClient, systemUUID string, verbose bool) error {
	_, err := restClient.GetLogicalPartitions(systemUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to get Logical partitions, with error %v", err)
	}
	return err
}
func GetMagdSystemQuick(restClient *hmc.HmcRestClient, systemUUID string, verbose bool) error {
	_, err := restClient.GetManagedSystemQuick(systemUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to get Manged Systems Quick, with error %v", err)
	}

	return err
}
func GetMagdSystemsQuick(restClient *hmc.HmcRestClient, verbose bool) error {
	_, err := restClient.GetManagedSystemsQuick(verbose)
	if err != nil {
		log.Fatalf("Failed to get Manged Systems, with error %v", err)
	}
	return err
}
func GetMagdSystemsByName(restClient *hmc.HmcRestClient, systemName,verbose bool) error {
	_, err := restClient.GetManagedSystemsByName(systemName,verbose)
	if err != nil {
		log.Fatalf("Failed to get Manged Systems, with error %v", err)
	}
	return err
}
func TransormTemp(restClient *hmc.HmcRestClient, tempUUID, systemUUID string, verbose bool) error {
	_, err := restClient.TransformPartitionTemplate(tempUUID, systemUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to Transform Template: %s, on Managed Systam: %s, with error %v", tempUUID, systemUUID, err)

	}
	return err
}
func CheckTemp(restClient *hmc.HmcRestClient, tempTemplateName, systemUUID string, verbose bool) error {
	_, err := restClient.CheckPartitionTemplate(tempTemplateName, systemUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to Check Template: %s, on Managed Systam: %s, with error %v", tempTemplateName, systemUUID, err)
	}
	return err
}
func PartitionPowerOn(restClient *hmc.HmcRestClient, systemUUID, lparUUID, profileUUID, keylock, iIPLsource, osType string, verbose bool) (string, error) {
	_, err := restClient.PowerOnPartition(systemUUID, lparUUID, profileUUID, "manual", "", osType, verbose)
	if err != nil {
		log.Fatalf("Failed to PowerOn Partition: %s, on Managed Systam: %s, with error %v", lparUUID, systemUUID, err)
	}
	log.Printf("rebooted successfully")

	return "", nil

}
func PowerOff(restClient *hmc.HmcRestClient, systemUUID, lparUUID, shutdownOption string, restart bool, verbose bool) (*etree.Document, error) {

	doc, err := restClient.PowerOffPartition(systemUUID, lparUUID, shutdownOption, restart, verbose)
	if err != nil {
		log.Fatalf("Failed to Restart Partition: %s, on Managed Systam: %s, with error %v", lparUUID, systemUUID, err)
	}
	return doc, err
}
func DeleteTempPartitionTemplate(restClient *hmc.HmcRestClient, templateName string, verbose bool) {
	err := restClient.DeletePartitionTemplate(templateName, verbose)
	if err != nil {
		log.Fatalf("Failed to Delete Partition Template: %s, with error %v", templateName, err)
	}
}
func getMacAdddress(restClient *hmc.HmcRestClient, systemUUID string, partUUID string, verbose bool) (string, error) {
	clientNetAdapter, err := restClient.GetClientNetworkAdapter(systemUUID, partUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to retrieve client network adapter: %v", err)
	}
	clineMacAddress := clientNetAdapter.FindElement("//MACAddress")
	return clineMacAddress.Text(), nil

}
func identifyFreeVolume(restClient *hmc.HmcRestClient, systemUUID string, volConfig hmc.VolumeConfig, VdiskUID string, verbose bool) (string, error) {
	viosName := volConfig.ViosName
	volumeName := volConfig.VolumeName

	// Get VIOS UUID using GetViosID
	viosUUID, err := hmc.GetViosID(restClient, systemUUID, viosName, verbose)
	if err != nil {
		return "", err
	}

	// Get free physical volumes for the VIOS
	pvList, err := restClient.GetFreePhyVolume(viosUUID, verbose)
	if err != nil {
		// Log the error and assume no volumes are available
		if verbose {
			log.Printf("Error getting free physical volumes for VIOS %s: %v", viosName, err)
		}
		pvList = []hmc.PhysicalVolume{} // Treat as no volumes found
	}

	// Find the volume with the given name
	for _, pv := range pvList {
		if strings.Contains(pv.VolumeUniqueID, VdiskUID) {
			if verbose {
				log.Printf("Found volume %s on VIOS %s", pv.VolumeName, viosName)
			}
			return pv.VolumeName, nil
		}
	}

	// If no volumes are found or the specific volume is not in the list
	if len(pvList) == 0 {
		return "", fmt.Errorf("no free physical volumes found on VIOS %s", viosName)
	}
	return "", fmt.Errorf("volume %s not found among free physical volumes on VIOS %s", volumeName, viosName)
}
