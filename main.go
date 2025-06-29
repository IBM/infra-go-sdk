package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

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

	// Handle managed system operations if system-name is provided
	var systemUUID string
	configDict := make(map[string]string) // Initialize configDict
	if *systemName != "" {
		uuid, _, err := restClient.GetManagedSystem(*systemName, *verbose)
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

		// Check service pack level and get next partition ID if < 951
		spLevelStr, ok := version["SERVICEPACK"]
		if !ok {
			log.Fatalf("SERVICEPACK not found in HMC version")
		}
		spLevel, err := strconv.Atoi(spLevelStr)
		if err != nil {
			log.Fatalf("Failed to parse SERVICEPACK level: %v", err)
		}
		if spLevel < 951 {
			// Fetch MaximumPartitions for the system
			if *verbose {
				log.Printf("Fetching MaximumPartitions for system UUID: %s", systemUUID)
			}
			maxLpars, err := restClient.GetMaximumPartitions(systemUUID, *verbose)
			if err != nil {
				log.Fatalf("Failed to fetch MaximumPartitions for system %s: %v", systemUUID, err)
			}
			maxLparsInt, err := strconv.Atoi(maxLpars)
			if err != nil {
				log.Fatalf("Failed to parse MaximumPartitions: %v", err)
			}
			nextLparID, err := hmcObj.GetNextPartitionID(*systemName, maxLparsInt, *verbose)
			if err != nil {
				log.Fatalf("Failed to get next partition ID: %v", err)
			}
			configDict["lpar_id"] = strconv.Itoa(nextLparID)
			if *verbose {
				log.Printf("Next Partition ID: %d", nextLparID)
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

		// Retrieve the copied template's UUID
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

		// Fetch MaximumPartitions for the system
		if *verbose {
			log.Printf("Fetching MaximumPartitions for system UUID: %s", systemUUID)
		}
		maxLpars, err := restClient.GetMaximumPartitions(systemUUID, *verbose)
		if err != nil {
			log.Fatalf("Failed to fetch MaximumPartitions for system %s: %v", systemUUID, err)
		}
		fmt.Printf("Maximum Partitions for system %s: %s\n", systemUUID, maxLpars)

		// Create a partition using the template
		if *verbose {
			log.Printf("Creating partition for system %s using template %s", systemUUID, tempTemplateName)
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
