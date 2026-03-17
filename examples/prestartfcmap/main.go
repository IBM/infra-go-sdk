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

	// Create a FlashCopyMappingID instance
	mapping := svc.FlashCopyMappingID{
		ID: "test_fcmap",
	}

	// Prepare the FlashCopy mapping
	if err := client.Prestartfcmap(mapping); err != nil {
		log.Fatalf("Prestartfcmap error: %v", err)
	} else {
		fmt.Printf("Successfully prepared FlashCopy mapping with ID: %s\n", mapping.ID)
	}
}
