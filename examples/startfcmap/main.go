package main

import (
	"fmt"
	"log"

	"github.com/sudeeshjohn/svc-go-sdk"
)

func main() {
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// Check if the FlashCopy mapping exists
	mappingName := "test_fcmap"
	mappings, err := client.Lsfcmap(mappingName)
	if err != nil {
		log.Fatalf("Lsfcmap error: %v", err)
	}
	if len(mappings) == 0 {
		log.Fatalf("No FlashCopy mapping found with name: %s", mappingName)
	}
	fmt.Printf("Found FlashCopy mapping: %s\n", mappingName)

	// Create a FlashCopyMappingStart instance
	//id := mappings[0].ID
	//Id, err := strconv.Atoi(id)
	mapping := svc.FlashCopyMappingStart{
		ID:      mappingName,
		Prep:    true, // Prepare the mapping before starting
		Restore: true, // Force start if target is in use
	}

	// Start the FlashCopy mapping
	if err := client.Startfcmap(mapping); err != nil {
		log.Fatalf("Startfcmap error: %v", err)
	} else {
		fmt.Printf("Successfully started FlashCopy mapping with ID: %s\n", mappings[0].Name)
	}
}
