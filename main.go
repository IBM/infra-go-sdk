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
	flag.Parse()

	if *hmcIP == "" || *username == "" || *password == "" {
		log.Fatal("All flags --hmc-ip, --username, and --password are required")
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
		var sourceTemplateName string
		switch *osType {
		case "aix", "linux", "aix_linux":
			sourceTemplateName = "AIX_Default" // Replace with valid template name from Step 1
		case "ibmi":
			sourceTemplateName = "IBMi_Default" // Replace with valid IBMi template name
		default:
			log.Fatalf("Invalid os-type: %s. Must be aix, linux, aix_linux, or ibmi", *osType)
		}

		destTemplateName := fmt.Sprintf("ansible_powervm_create_%04d", rand.Intn(9000)+1000)
		if *verbose {
			log.Printf("Generated destination template name: %s", destTemplateName)
		}

		if *verbose {
			log.Printf("Retrieving AtomID for os-type %s (template: %s)", *osType, sourceTemplateName)
		}
		sourceID, err := hmc.GetPartitionTemplateID(client, *hmcIP, session, sourceTemplateName, *verbose)
		if err != nil {
			log.Fatalf("Failed to get template ID for %s: %v", sourceTemplateName, err)
		}
		fmt.Printf("Source Template ID for os-type %s (template %s): %s\n", *osType, sourceTemplateName, sourceID)

		if *verbose {
			log.Printf("Copying template from %s to %s", sourceTemplateName, destTemplateName)
		}
		err = hmc.CopyPartitionTemplate(client, *hmcIP, session, sourceTemplateName, destTemplateName, *verbose)
		if err != nil {
			log.Fatalf("Failed to copy template from %s to %s: %v", sourceTemplateName, destTemplateName, err)
		}
		fmt.Printf("Successfully copied template from %s to %s\n", sourceTemplateName, destTemplateName)

		if *verbose {
			log.Printf("Retrieving AtomID for destination template: %s", destTemplateName)
		}
		destID, err := hmc.GetPartitionTemplateID(client, *hmcIP, session, destTemplateName, *verbose)
		if err != nil {
			log.Printf("Warning: Failed to get template ID for %s: %v", destTemplateName, err)
		} else {
			fmt.Printf("Destination Template ID for %s: %s\n", destTemplateName, destID)
		}
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
