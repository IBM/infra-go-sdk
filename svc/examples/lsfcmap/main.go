package main

import (
	"log"
	"context"
	"flag"
	"os"
	"strconv"

	"github.com/IBM/infra-go-sdk/svc" // Adjust if your package path differs
)

func main() {
	// Command line flags
	verbose := flag.Bool("verbose", false, "Enable verbose output to see detailed mappings")
	svcIP := flag.String("svc-ip", "", "SVC IP address (required)")
	svcUser := flag.String("svc-user", "", "SVC username (required)")
	svcPass := flag.String("svc-pass", "", "SVC password (required)")
	flag.Parse()

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: lsfcmap -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()

	// Initialize the client
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	// Enable debug logging if the verbose flag is passed
	if *verbose {
		log.Printf("Verbose mode enabled. Connecting to SVC.: ip=%v user=%v", *svcIP, *svcUser)
	}

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}
	log.Println("✅ Authenticated")

	// --- 1. Search for a specific FlashCopy mapping by name ---
	mappingName := "test_fcmap"
	log.Printf("Searching for FlashCopy mapping...: target=%v", mappingName)

	mappings, err := client.Lsfcmap(ctx,mappingName)
	if err != nil {
		log.Printf("Lsfcmap error: error=%v", err)
		os.Exit(1)
	}

	if len(mappings) > 0 {
		mapping := mappings[0]
		
		// Parse the copy rate safely
		copyRate, _ := strconv.Atoi(mapping.CopyRate)
		if mapping.CopyRate == "" {
			copyRate = 0
		}

		log.Printf("✅ Successfully retrieved FlashCopy mapping: name=%v", mapping.Name)
		log.Printf("[DEBUG] Mapping Details %v", "id", mapping.ID,
			"source", mapping.SourceVDiskName,
			"target", mapping.TargetVDiskName,
			"status", mapping.Status,
			"copy_rate", copyRate,
			"source_vdisk_id", mapping.SourceVDiskID,)
	} else {
		log.Printf("No FlashCopy mapping found: name=%v", mappingName)
	}

	// --- 2. List all FlashCopy mappings ---
	log.Println("Fetching all FlashCopy mappings...")
	
	allMappings, err := client.Lsfcmap(ctx,"")
	if err != nil {
		log.Printf("Lsfcmap error for all mappings: error=%v", err)
		os.Exit(1)
	}

	if len(allMappings) == 0 {
		log.Println("No FlashCopy mappings found on the system")
	} else {
		log.Printf("Retrieved all FlashCopy mappings: total_mappings=%v", len(allMappings))
		
		for _, mapping := range allMappings {
			// Parse the copy rate safely
			copyRate, _ := strconv.Atoi(mapping.CopyRate)
			if mapping.CopyRate == "" {
				copyRate = 0
			}

			// We use Debug here so the console isn't flooded unless -verbose is used
			log.Printf("[DEBUG] Mapping Detail %v", "name", mapping.Name,
				"source", mapping.SourceVDiskName,
				"target", mapping.TargetVDiskName,
				"status", mapping.Status,
				"copy_rate", copyRate,)
		}
	}
}