package main

import (
	"flag"
	"os"
	"strconv"

	"github.com/sudeeshjohn/svc-go-sdk" // Adjust if your package path differs
)

func main() {
	// Command line flags
	verbose := flag.Bool("verbose", false, "Enable verbose output to see detailed mappings")
	svcIP := flag.String("svc-ip", "REDACTED_SVC_IP<==", "SVC IP address")
	svcUser := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC username")
	svcPass := flag.String("svc-pass", "REDACTED_SVC_PASS<==", "SVC password")
	flag.Parse()

	// Initialize the client
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	// Enable debug logging if the verbose flag is passed
	if *verbose {
		client = client.WithDebug()
		client.Logger.Debug("Verbose mode enabled. Connecting to SVC.", "ip", *svcIP, "user", *svcUser)
	}

	if err := client.Authenticate(); err != nil {
		client.Logger.Error("Authentication error", "error", err)
		os.Exit(1)
	}
	client.Logger.Info("✅ Authenticated")

	// --- 1. Search for a specific FlashCopy mapping by name ---
	mappingName := "test_fcmap"
	client.Logger.Info("Searching for FlashCopy mapping...", "target", mappingName)

	mappings, err := client.Lsfcmap(mappingName)
	if err != nil {
		client.Logger.Error("Lsfcmap error", "error", err)
		os.Exit(1)
	}

	if len(mappings) > 0 {
		mapping := mappings[0]
		
		// Parse the copy rate safely
		copyRate, _ := strconv.Atoi(mapping.CopyRate)
		if mapping.CopyRate == "" {
			copyRate = 0
		}

		client.Logger.Info("✅ Successfully retrieved FlashCopy mapping", "name", mapping.Name)
		client.Logger.Debug("Mapping Details",
			"id", mapping.ID,
			"source", mapping.SourceVDiskName,
			"target", mapping.TargetVDiskName,
			"status", mapping.Status,
			"copy_rate", copyRate,
			"source_vdisk_id", mapping.SourceVDiskID,
		)
	} else {
		client.Logger.Warn("No FlashCopy mapping found", "name", mappingName)
	}

	// --- 2. List all FlashCopy mappings ---
	client.Logger.Info("Fetching all FlashCopy mappings...")
	
	allMappings, err := client.Lsfcmap("")
	if err != nil {
		client.Logger.Error("Lsfcmap error for all mappings", "error", err)
		os.Exit(1)
	}

	if len(allMappings) == 0 {
		client.Logger.Info("No FlashCopy mappings found on the system")
	} else {
		client.Logger.Info("Retrieved all FlashCopy mappings", "total_mappings", len(allMappings))
		
		for _, mapping := range allMappings {
			// Parse the copy rate safely
			copyRate, _ := strconv.Atoi(mapping.CopyRate)
			if mapping.CopyRate == "" {
				copyRate = 0
			}

			// We use Debug here so the console isn't flooded unless -verbose is used
			client.Logger.Debug("Mapping Detail",
				"name", mapping.Name,
				"source", mapping.SourceVDiskName,
				"target", mapping.TargetVDiskName,
				"status", mapping.Status,
				"copy_rate", copyRate,
			)
		}
	}
}