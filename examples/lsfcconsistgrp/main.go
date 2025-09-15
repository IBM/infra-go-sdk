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

	// Search for a specific FlashCopy consistency group by name
	groupName := "test_fcgrp"
	groups, err := client.Lsfcconsistgrp(groupName)
	if err != nil {
		log.Fatalf("Lsfcconsistgrp error: %v", err)
	}

	if len(groups) > 0 {
		group := groups[0]
		fmt.Printf("Successfully retrieved FlashCopy consistency group with name: %s\n", group.Name)
		fmt.Printf("Details: Status: %s, Start Time: %sn", group.Status, group.StartTime)
		if len(group.Mappings) > 0 {
			fmt.Println("Associated Mappings:")
			for _, mapping := range group.Mappings {
				fmt.Printf("  Mapping ID: %s, Name: %s\n", mapping.FCMappingID, mapping.FCMappingName)
			}
		} else {
			fmt.Println("Associated Mappings: None")
		}
	} else {
		fmt.Printf("No FlashCopy consistency group found with name: %s\n", groupName)
	}

	// List all FlashCopy consistency groups
	fmt.Println("\nAll FlashCopy Consistency Groups:")
	allGroups, err := client.Lsfcconsistgrp("")
	if err != nil {
		log.Fatalf("Lsfcconsistgrp error for all groups: %v", err)
	}
	if len(allGroups) == 0 {
		fmt.Println("No FlashCopy consistency groups found")
	} else {
		for _, group := range allGroups {
			fmt.Printf("Name: %s, Status: %s, Start Time: %s\n", group.Name, group.Status, group.StartTime)
			if len(group.Mappings) > 0 {
				fmt.Println("  Associated Mappings:")
				for _, mapping := range group.Mappings {
					fmt.Printf("    Mapping ID: %s, Name: %s\n", mapping.FCMappingID, mapping.FCMappingName)
				}
			} else {
				fmt.Println("  Associated Mappings: None")
			}
		}
	}
}
