package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/mkumatag/svc-go-sdk"
)

func main() {
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// Search for a specific FlashCopy mapping by name
	mappingName := "test_fcmap"
	mappings, err := client.Lsfcmap(mappingName)
	if err != nil {
		log.Fatalf("Lsfcmap error: %v", err)
	}

	if len(mappings) > 0 {
		mapping := mappings[0]
		copyRate, _ := strconv.Atoi(mapping.CopyRate)
		if mapping.CopyRate == "" {
			copyRate = 0
		}
		fmt.Printf("Successfully retrieved FlashCopy mapping with name: %s\n", mapping.Name)
		fmt.Printf("Details: ID: %s, Source: %s, Target: %s, Status: %s, Copy Rate: %d, Source Vdisk ID: %s\n",
			mapping.ID, mapping.SourceVDiskName, mapping.TargetVDiskName, mapping.Status, copyRate, mapping.SourceVDiskID)
	} else {
		fmt.Printf("No FlashCopy mapping found with name: %s\n", mappingName)
	}

	// List all FlashCopy mappings
	fmt.Println("\nAll FlashCopy Mappings:")
	allMappings, err := client.Lsfcmap("")
	if err != nil {
		log.Fatalf("Lsfcmap error for all mappings: %v", err)
	}
	if len(allMappings) == 0 {
		fmt.Println("No FlashCopy mappings found")
	} else {
		for _, mapping := range allMappings {
			copyRate, _ := strconv.Atoi(mapping.CopyRate)
			if mapping.CopyRate == "" {
				copyRate = 0
			}
			fmt.Printf("Name: %s, Source: %s, Target: %s, Status: %s, Copy Rate: %d\n",
				mapping.Name, mapping.SourceVDiskName, mapping.TargetVDiskName, mapping.Status, copyRate)
		}
	}
}
