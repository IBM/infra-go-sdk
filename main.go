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
	systemUUID := flag.String("system-uuid", "", "System UUID for CEC (required for copying)")
	flag.Parse()

	if *hmcIP == "" || *username == "" || *password == "" {
		log.Fatal("All flags --hmc-ip, --username, and --password are required")
	}
	if *osType != "" && *systemUUID == "" {
		log.Fatal("Flag --system-uuid is required when --os-type is specified")
	}

	// Create HTTP client with insecure SSL
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	// Logon
	if *verbose {
		log.Printf("Attempting to log on to HMC at %s with username %s", *hmcIP, *username)
	}
	session, err := hmc.Logon(client, *hmcIP, *username, *password, *verbose)
	if err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	if *verbose {
		log.Printf("Logon successful, session token: %s", session)
	}

	// List all partition template IDs if --list-template is set
	if *listTemplate {
		if *verbose {
			log.Printf("Listing all partition template IDs")
		}
		ids, err := hmc.ListPartitionTemplateIDs(client, *hmcIP, session, *verbose)
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
		id, err := hmc.GetPartitionTemplateID(client, *hmcIP, session, *templateName, *verbose)
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
			referenceTemplate = "QuickStart_lpar_rpa_2" // Replace with valid template name from --list-template
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
		err = hmc.CopyPartitionTemplate(client, *hmcIP, session, referenceTemplate, tempTemplateName, *verbose)
		if err != nil {
			log.Fatalf("Failed to copy template from %s to %s: %v", referenceTemplate, tempTemplateName, err)
		}
		fmt.Printf("Successfully copied template from %s to %s\n", referenceTemplate, tempTemplateName)

		// Retrieve the copied template's UUID
		if *verbose {
			log.Printf("Retrieving AtomID for temporary template: %s", tempTemplateName)
		}
		tempTemplateDoc, err := hmc.GetPartitionTemplate(client, *hmcIP, session, "", tempTemplateName, *verbose)
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
			log.Printf("Fetching MaximumPartitions for system UUID: %s", *systemUUID)
		}
		maxLpars, err := hmc.GetMaximumPartitions(client, *hmcIP, session, *systemUUID, *verbose)
		if err != nil {
			log.Fatalf("Failed to fetch MaximumPartitions for system %s: %v", *systemUUID, err)
		}
		fmt.Printf("Maximum Partitions for system %s: %s\n", *systemUUID, maxLpars)
	}

	// Logoff
	if *verbose {
		log.Printf("Attempting to log off")
	}
	if err := hmc.Logoff(client, *hmcIP, session, *verbose); err != nil {
		log.Fatalf("Logoff failed: %v", err)
	}
	if *verbose {
		log.Println("Logged off successfully")
	}
}
