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

	// Create a FlashCopyConsistGroup instance
	group := svc.FlashCopyConsistGroup{
		Name:       "test_fcgrp",
		AutoDelete: false,
	}

	// Create the FlashCopy consistency group
	if err := client.Mkfcconsistgrp(group); err != nil {
		log.Fatalf("Mkfcconsistgrp error: %v", err)
	} else {
		fmt.Printf("Successfully created FlashCopy consistency group with name: %s\n", group.Name)
	}
}
