package main

import (
	"log"
	"context"
	"flag"
	"os"

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
		log.Fatal("Usage: lsfcconsistgrp -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
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

	// --- 1. Search for a specific FlashCopy consistency group ---
	groupName := "test_fcgrp"
	log.Printf("Searching for FlashCopy consistency group...: target=%v", groupName)
	
	groups, err := client.Lsfcconsistgrp(ctx,groupName)
	if err != nil {
		log.Printf("Lsfcconsistgrp error: error=%v", err)
		os.Exit(1)
	}

	if len(groups) > 0 {
		group := groups[0]
		log.Printf("[INFO] ✅ Successfully retrieved FlashCopy consistency group %v", "name", group.Name, 
			"status", group.Status, 
			"start_time", group.StartTime,)
		
		if len(group.Mappings) > 0 {
			log.Printf("Associated Mappings found: count=%v", len(group.Mappings))
			for _, mapping := range group.Mappings {
				log.Printf("[DEBUG] Mapping Detail %v", "mapping_id", mapping.FCMappingID, 
					"mapping_name", mapping.FCMappingName,)
			}
		} else {
			log.Println("No associated mappings for this group")
		}
	} else {
		log.Printf("No FlashCopy consistency group found: name=%v", groupName)
	}

	// --- 2. List all FlashCopy consistency groups ---
	log.Println("Fetching all FlashCopy consistency groups...")
	allGroups, err := client.Lsfcconsistgrp(ctx,"")
	if err != nil {
		log.Printf("Lsfcconsistgrp error for all groups: error=%v", err)
		os.Exit(1)
	}

	if len(allGroups) == 0 {
		log.Println("No FlashCopy consistency groups found on the system")
	} else {
		log.Printf("Retrieved all FlashCopy consistency groups: total_groups=%v", len(allGroups))
		
		for _, group := range allGroups {
			log.Printf("[DEBUG] Consistency Group %v", "name", group.Name, 
				"status", group.Status, 
				"start_time", group.StartTime,)
			
			if len(group.Mappings) > 0 {
				for _, mapping := range group.Mappings {
					log.Printf("[DEBUG]   -> Mapping Detail %v", "parent_group", group.Name,
						"mapping_id", mapping.FCMappingID, 
						"mapping_name", mapping.FCMappingName,)
				}
			}
		}
	}
}