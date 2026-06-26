package main

import (
	"context"
	"flag"
	"os"

	"github.ibm.com/sudeeshjohn/infra-go-sdk/svc" // Adjust if your package path differs
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
		client = client.WithDebug()
		client.Logger.Debug("Verbose mode enabled. Connecting to SVC.", "ip", *svcIP, "user", *svcUser)
	}

	if err := client.Authenticate(ctx); err != nil {
		client.Logger.Error("Authentication error", "error", err)
		os.Exit(1)
	}
	client.Logger.Info("✅ Authenticated")

	// --- 1. Search for a specific FlashCopy consistency group ---
	groupName := "test_fcgrp"
	client.Logger.Info("Searching for FlashCopy consistency group...", "target", groupName)
	
	groups, err := client.Lsfcconsistgrp(ctx,groupName)
	if err != nil {
		client.Logger.Error("Lsfcconsistgrp error", "error", err)
		os.Exit(1)
	}

	if len(groups) > 0 {
		group := groups[0]
		client.Logger.Info("✅ Successfully retrieved FlashCopy consistency group", 
			"name", group.Name, 
			"status", group.Status, 
			"start_time", group.StartTime,
		)
		
		if len(group.Mappings) > 0 {
			client.Logger.Debug("Associated Mappings found", "count", len(group.Mappings))
			for _, mapping := range group.Mappings {
				client.Logger.Debug("Mapping Detail", 
					"mapping_id", mapping.FCMappingID, 
					"mapping_name", mapping.FCMappingName,
				)
			}
		} else {
			client.Logger.Debug("No associated mappings for this group")
		}
	} else {
		client.Logger.Warn("No FlashCopy consistency group found", "name", groupName)
	}

	// --- 2. List all FlashCopy consistency groups ---
	client.Logger.Info("Fetching all FlashCopy consistency groups...")
	allGroups, err := client.Lsfcconsistgrp(ctx,"")
	if err != nil {
		client.Logger.Error("Lsfcconsistgrp error for all groups", "error", err)
		os.Exit(1)
	}

	if len(allGroups) == 0 {
		client.Logger.Info("No FlashCopy consistency groups found on the system")
	} else {
		client.Logger.Info("Retrieved all FlashCopy consistency groups", "total_groups", len(allGroups))
		
		for _, group := range allGroups {
			client.Logger.Debug("Consistency Group", 
				"name", group.Name, 
				"status", group.Status, 
				"start_time", group.StartTime,
			)
			
			if len(group.Mappings) > 0 {
				for _, mapping := range group.Mappings {
					client.Logger.Debug("  -> Mapping Detail", 
						"parent_group", group.Name,
						"mapping_id", mapping.FCMappingID, 
						"mapping_name", mapping.FCMappingName,
					)
				}
			}
		}
	}
}