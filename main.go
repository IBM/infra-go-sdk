package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/sudeeshjohn/PowerHMC/pkg/hmc"
	"golang.org/x/crypto/ssh"
)

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

	// Validate required flags
	if *hmcIP == "" || *username == "" || *password == "" {
		log.Fatal("All flags --hmc-ip, --username, and --password are required")
	}
	if *osType != "" && *systemName == "" {
		log.Fatal("Flag --system-name is required when --os-type is specified")
	}

	// SSH Connection for CLI operations
	sshConfig := &ssh.ClientConfig{
		User: *username,
		Auth: []ssh.AuthMethod{
			ssh.Password(*password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	sshClient, err := ssh.Dial("tcp", *hmcIP+":22", sshConfig)
	if err != nil {
		log.Fatalf("Failed to connect to HMC via SSH: %v", err)
	}
	defer sshClient.Close()

	// Initialize HMC object
	hmcObj := hmc.NewHmc(sshClient)
	version, err := hmcObj.ListHMCVersion(*verbose)
	if err != nil {
		log.Fatalf("Failed to list HMC version: %v", err)
	}
	fmt.Printf("HMC Version: %+v\n", version)

	// Create HTTP client with insecure SSL
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Initialize HmcRestClient
	restClient := hmc.NewHmcRestClient(*hmcIP, client)

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
		var referenceTemplate string
		switch *osType {
		case "aix", "linux", "aix_linux":
			referenceTemplate = "QuickStart_lpar_rpa_2" // Replace with valid template name
		case "ibmi":
			referenceTemplate = "QuickStart_lpar_IBMi_2" // Replace with valid IBMi template name
		default:
			log.Fatalf("Invalid os-type: %s. Must be aix, linux, aix_linux, or ibmi", *osType)
		}

		// Generate a unique temporary template name
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		tempTemplateName := fmt.Sprintf("ansible_powervm_create_%04d", rng.Intn(9000)+1000)
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

		// Print the updated XML for verification
		if *verbose {
			doc := etree.NewDocument()
			doc.SetRoot(tempTemplateDoc)
			xmlString, err := doc.WriteToString()
			if err != nil {
				log.Printf("Failed to serialize updated XML: %v", err)
			} else {
				log.Printf("Updated XML:\n%s", xmlString)
			}
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
				log.Printf("Updated XML:\n%s", xmlString)
			}
		}
		// Update the partition template with the modified XML
		if *verbose {
			log.Printf("Updating partition template with UUID: %s", tempUUID)
		}
		if err := restClient.UpdatePartitionTemplate(tempUUID, tempTemplateDoc, *verbose); err != nil {
			log.Fatalf("Failed to update partition template: %v", err)
		}
		fmt.Printf("Successfully updated partition template with UUID: %s\n", tempUUID)
		// Fetch MaximumPartitions for the system
		if *verbose {
			log.Printf("Fetching MaximumPartitions for system UUID: %s", systemUUID)
		}
		maxLpars, err := restClient.GetMaximumPartitions(systemUUID, *verbose)
		if err != nil {
			log.Fatalf("Failed to fetch MaximumPartitions for system %s: %v", systemUUID, err)
		}
		fmt.Printf("Maximum Partitions for system %s: %s\n", systemUUID, maxLpars)

		// Create a partition using the updated template
		if *verbose {
			log.Printf("Creating partition for system %s using updated template %s", systemUUID, tempTemplateName)
		}
		jobID, err := restClient.CreatePartition(systemUUID, tempUUID, *osType, *verbose)
		if err != nil {
			log.Fatalf("Failed to create partition: %v", err)
		}
		fmt.Printf("Partition creation job ID: %s\n", jobID)

		// Check job status
		if *verbose {
			log.Printf("Checking status for job ID: %s", jobID)
		}
		for i := 0; i < 10; i++ { // Retry up to 10 times
			time.Sleep(5 * time.Second)
			status, err := restClient.FetchJobStatus(jobID, true, *verbose)
			if err != nil {
				log.Fatalf("Failed to check job status: %v", err)
			}
			if *verbose {
				log.Printf("Job status: %s", status)
			}
			if status == "COMPLETED" {
				fmt.Printf("Partition creation completed successfully\n")
				break
			}
			if status == "FAILED" || status == "COMPLETED_WITH_ERRORS" {
				log.Fatalf("Partition creation failed with status: %s", status)
			}
		}

		// Log configDict if verbose
		if *verbose && len(configDict) > 0 {
			log.Printf("Configuration dictionary: %+v", configDict)
		}
	}
}
