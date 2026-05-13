package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	systemName := flag.String("system-name", "", "Managed system name (required)")
	lparName := flag.String("lpar-name", "", "LPAR name (required)")
	profileName := flag.String("profile-name", "", "Profile name to set as default (required)")
	verbose := flag.Bool("verbose", true, "Enable verbose logging")
	timeout := flag.Int("timeout", 10, "Timeout in minutes for job completion")

	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel() // Automatically cleans up the timer/goroutine the second the function exits
	//req := req.WithContext(ctx)

	// Validate required parameters
	if *systemName == "" || *lparName == "" || *profileName == "" {
		log.Fatal("Error: -system-name, -lpar-name, and -profile-name are all required\n\n" +
			"Usage: go run main.go -system-name=<system> -lpar-name=<lpar> -profile-name=<profile>\n" +
			"Example: go run main.go -system-name=Server-9080-MHE-SN1234567 -lpar-name=mylpar -profile-name=default_profile")
	}

	fmt.Println("=== Change Default Profile Name Example ===")
	fmt.Println("This example demonstrates how to change the default profile of a logical partition")
	fmt.Println()

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// RESOLVE SYSTEM UUID
	// =========================================================================
	fmt.Printf("Resolving managed system: %s\n", *systemName)
	
	systemUUID, system, err := restClient.GetManagedSystemByName(context.Background(), *systemName, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get managed system: %v", err)
	}
	
	fmt.Printf("✅ Found system: %s (UUID: %s)\n", system.SystemName, systemUUID)
	fmt.Println()

	// =========================================================================
	// RESOLVE LPAR UUID
	// =========================================================================
	fmt.Printf("Resolving LPAR: %s\n", *lparName)
	
	lpar, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), systemUUID, *lparName, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get LPAR: %v", err)
	}
	
	fmt.Printf("✅ Found LPAR: %s (UUID: %s)\n", lpar.PartitionName, lparUUID)
	fmt.Printf("   Current State: %s\n", lpar.PartitionState)
	fmt.Printf("   Partition Type: %s\n", lpar.PartitionType)
	fmt.Println()

	// =========================================================================
	// CHANGE DEFAULT PROFILE
	// =========================================================================
	fmt.Printf("Changing default profile to: %s\n", *profileName)
	fmt.Println()

	// Submit the job
	jobID, err := restClient.ChangeDefaultProfileName(lparUUID, *profileName, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to submit ChangeDefaultProfileName job: %v", err)
	}

	fmt.Printf("✅ Job submitted successfully!\n")
	fmt.Printf("Job ID: %s\n", jobID)
	fmt.Println()

	// =========================================================================
	// MONITOR JOB STATUS
	// =========================================================================
	fmt.Println("Monitoring job status...")
	fmt.Println()

	jobResp, err := restClient.FetchJobStatus(ctx,jobID, false, *timeout, *verbose)
	if err != nil {
		log.Fatalf("❌ Job failed: %v", err)
	}

	// =========================================================================
	// DISPLAY RESULTS
	// =========================================================================
	fmt.Println("✅ Default profile changed successfully!")
	fmt.Printf("Job Status: %s\n", jobResp.Status)
	
	if jobResp.TimeStarted != "" {
		fmt.Printf("Time Started: %s\n", jobResp.TimeStarted)
	}
	if jobResp.TimeCompleted != "" {
		fmt.Printf("Time Completed: %s\n", jobResp.TimeCompleted)
	}

	if len(jobResp.Results.Parameters) > 0 {
		fmt.Println("\nJob Results:")
		for _, param := range jobResp.Results.Parameters {
			fmt.Printf("  %s: %s\n", param.ParameterName, param.ParameterValue)
		}
	}

	fmt.Println()
	fmt.Printf("The default profile for LPAR '%s' has been changed to '%s'.\n", *lparName, *profileName)
	fmt.Println("This profile will be used when the partition is next activated without specifying a profile.")

	// =========================================================================
	// CLEANUP JOB
	// =========================================================================
	fmt.Println()
	fmt.Println("Cleaning up job...")
	if err := restClient.DeleteJob(context.Background(), jobID, false, *verbose); err != nil {
		log.Printf("⚠️  Warning: Failed to delete job: %v", err)
	} else {
		fmt.Println("✅ Job deleted successfully")
	}
}

// Made with Bob
