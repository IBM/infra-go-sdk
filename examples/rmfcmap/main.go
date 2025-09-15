package main

import (
	"fmt"
	"log"

	"github.com/mkumatag/svc-go-sdk"
)

func main() {
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// Create a FlashCopyMappingRemove instance
	mappingName := "test_fcmap"
	removeMapping := svc.FlashCopyMappingRemove{
		Force: true, // Force deletion if in stopped state
	}

	// Delete the FlashCopy mapping
	if err := client.Rmfcmap(mappingName, removeMapping); err != nil {
		log.Fatalf("Rmfcmap error: %v", err)
	} else {
		fmt.Printf("Successfully deleted FlashCopy mapping with name: %s\n", mappingName)
	}
}
