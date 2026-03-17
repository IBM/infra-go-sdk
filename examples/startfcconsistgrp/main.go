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

	// Placeholder: Validate if the FlashCopy consistency group exists and is in a valid state
	// Example (uncomment if Lsfcconsistgrp is implemented):
	/*
		groupName := "test_fcgrp"
		groups, err := client.Lsfcconsistgrp(groupName)
		if err != nil {
			log.Fatalf("Lsfcconsistgrp error: %v", err)
		}
		if len(groups) == 0 {
			log.Fatalf("No FlashCopy consistency group found with name: %s", groupName)
		}
		group := groups[0]
		if !strings.Contains(group.Status, "idle_or_copied") {
			log.Fatalf("FlashCopy consistency group %s is in invalid state: %s (expected idle_or_copied)", groupName, group.Status)
		}
		fmt.Printf("Found FlashCopy consistency group: %s (Status: %s)\n", groupName, group.Status)
	*/

	// Create a FlashCopyConsistGroupStart instance
	startGroup := svc.FlashCopyConsistGroupStart{
		ID:      "test_fcgrp",
		Prep:    true, // Prepare the group before starting
		Restore: true, // Force start if target is in use
	}

	// Start the FlashCopy consistency group
	if err := client.Startfcconsistgrp(startGroup); err != nil {
		log.Fatalf("Startfcconsistgrp error: %v", err)
	} else {
		fmt.Printf("Successfully started FlashCopy consistency group with name: %s\n", startGroup.ID)
	}
}
