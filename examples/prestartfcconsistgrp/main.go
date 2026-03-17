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

	// Create a FlashCopyConsistGroupID instance
	group := svc.FlashCopyConsistGroupID{
		ID: "test_fcgrp",
	}

	// Prepare the FlashCopy consistency group
	if err := client.Prestartfcconsistgrp(group); err != nil {
		log.Fatalf("Prestartfcconsistgrp error: %v", err)
	} else {
		fmt.Printf("Successfully prepared FlashCopy consistency group with ID: %s\n", group.ID)
	}
}
