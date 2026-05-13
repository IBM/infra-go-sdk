package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS (Required)")
	mediaName := flag.String("media-name", "test_iso", "Name of the Virtual Optical Media to create in the repository")
	
	// Two mutually exclusive ways to create the media:
	sourceFile := flag.String("source-file", "/home/padmin/aixtest_iso.iso", "Path to an existing ISO file on the VIOS (e.g., /mnt/nfs/aix72.iso)")
	mediaSize := flag.Int("media-size", 9216, "Size of the blank media in MB (Used only if source-file is empty)")
	
	// Advanced Modifiers
	readOnly := flag.Bool("ro", false, "Set the media as read-only (highly recommended for imported ISOs)")
	nfsLink := flag.Bool("nfslink", false, "Create an NFS link to the file instead of copying it (Requires -source-file)")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	// Initial Validation
	if *password == "" || *viosName == "" || *mediaName == "" {
		log.Fatal("Error: hmc-pass, vios-name, and media-name are required.")
	}

	if *sourceFile == "" && *mediaSize <= 0 {
		log.Fatal("Error: You must provide either a valid -source-file path OR a -media-size greater than 0.")
	}

	// NFS Link Validation
	if *nfsLink && *sourceFile == "" {
		log.Fatal("Error: The -nfslink flag can only be used when providing a -source-file path.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// RESOLVE SYSTEM & VIOS UUID
	// =========================================================================
	fmt.Printf("\nResolving System Name: %s...\n", *sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found: %v", *sysName, err)
	}

	viosUUID, err := hmc.GetViosID(context.Background(), restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found on system '%s'.", *viosName, *sysName)
	}

	// =========================================================================
	// EXECUTE CREATION / IMPORT
	// =========================================================================
	if *sourceFile != "" {
		if *nfsLink {
			fmt.Printf("\n🚀 Attempting to LINK NFS ISO from '%s' into repository as '%s' on VIOS '%s'...\n", *sourceFile, *mediaName, *viosName)
		} else {
			fmt.Printf("\n🚀 Attempting to COPY ISO from '%s' into repository as '%s' on VIOS '%s'...\n", *sourceFile, *mediaName, *viosName)
		}
	} else {
		fmt.Printf("\n🚀 Attempting to create a %d MB blank Virtual Optical Media '%s' on VIOS '%s'...\n", *mediaSize, *mediaName, *viosName)
	}

	if *readOnly {
		fmt.Println("   🔒 Applying Read-Only (-ro) protection to the media.")
	}

	err = restClient.CreateVirtualOpticalMedia(context.Background(), *sysName, viosUUID, *viosName, *mediaName, *sourceFile, *mediaSize, *readOnly, *nfsLink, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to create/import Virtual Optical Media: %v", err)
	}

	fmt.Printf("\n💿 Successfully provisioned Virtual Optical Media '%s'!\n", *mediaName)
}
