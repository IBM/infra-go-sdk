package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
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

	hmcObj := hmc.NewHmc(sshClient)
	version, err := hmcObj.ListHMCVersion()
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

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	// Initialize HmcRestClient
	restClient := hmc.NewHmcRestClient(*hmcIP, client)

	// Logon
	if *verbose {
		log.Printf("Attempting to log on to HMC at %s with username %s", *hmcIP, *username)
	}
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff() // Ensure logoff on exit
	if *verbose {
		log.Printf("Logon successful, session token: %s", restClient.Session())
	}

	// Fetch managed system UUID if system-name is provided
	var systemUUID string
	if *systemName != "" {
		uuid, _, err := restClient.GetManagedSystem(*systemName)
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
	}

	// List all partition template IDs if --list-template is set
	if *listTemplate {
		if *verbose {
			log.Printf("Listing all partition template IDs")
		}
		ids, err := hmc.ListPartitionTemplateIDs(client, *hmcIP, restClient.Session(), *verbose)
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
		id, err := hmc.GetPartitionTemplateID(client, *hmcIP, restClient.Session(), *templateName, *verbose)
		if err != nil {
			log.Fatalf("Failed to get template ID for %s: %v", *templateName, err)
		}
		fmt.Printf("Template ID for %s: %s\n", *templateName, id)
	}

	// Perform template copy if os-type is set
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

		tempTemplateName := fmt.Sprintf("ansible_powervm_create_%04d", rand.Intn(9000)+1000)
		if *verbose {
			log.Printf("Generated temporary template name: %s", tempTemplateName)
		}

		// Copy the template
		if *verbose {
			log.Printf("Copying template from %s to %s", referenceTemplate, tempTemplateName)
		}
		err = hmc.CopyPartitionTemplate(client, *hmcIP, restClient.Session(), referenceTemplate, tempTemplateName, *verbose)
		if err != nil {
			log.Fatalf("Failed to copy template from %s to %s: %v", referenceTemplate, tempTemplateName, err)
		}
		fmt.Printf("Successfully copied template from %s to %s\n", referenceTemplate, tempTemplateName)

		// Retrieve the copied template's UUID
		if *verbose {
			log.Printf("Retrieving AtomID for temporary template: %s", tempTemplateName)
		}
		tempTemplateDoc, err := hmc.GetPartitionTemplate(client, *hmcIP, restClient.Session(), "", tempTemplateName, *verbose)
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
		maxLpars, err := hmc.GetMaximumPartitions(client, *hmcIP, restClient.Session(), systemUUID, *verbose)
		if err != nil {
			log.Fatalf("Failed to fetch MaximumPartitions for system %s: %v", systemUUID, err)
		}
		fmt.Printf("Maximum Partitions for system %s: %s\n", systemUUID, maxLpars)
	}

	// Logoff is handled by defer
	if *verbose {
		log.Println("Logged off successfully")
	}
}
