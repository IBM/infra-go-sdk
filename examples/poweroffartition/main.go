package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	lparName := flag.String("lpar-name", "sno-master", "Target LPAR Name")
	
	// PowerOff specific flags
	shutdownOpt := flag.String("shutdown-option", "Immediate", "Delayed, Immediate, OperatingSystem, OSImmediate, Dump, DumpRetry")
	restart := flag.Bool("restart", false, "Restart the partition after powering off")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel() // Automatically cleans up the timer/goroutine the second the function exits

	// Validation
	if *password == "" || *sysName == "" || *lparName == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, and lpar-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)

	if *verbose {
		log.Printf("Attempting to log on to HMC at %s with username %s", *hmcIP, *username)
	}
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		if *verbose {
			log.Fatalf("Logon failed: %v", err)
		}
		log.Fatal("❌ Logon failed. (Run with -verbose for details)")
	}
	defer func() {
		if err := restClient.Logoff(context.Background()); err != nil {
			if *verbose {
				log.Printf("Logoff failed: %v", err)
			}
		} else if *verbose {
			log.Println("Logged off successfully")
		}
	}()

	// =========================================================================
	// DYNAMIC RESOLUTION & STATE CHECK
	// =========================================================================
	
	// 1. Resolve System UUID from System Name 
	if *verbose {
		fmt.Printf("\nResolving System UUID for '%s'...\n", *sysName)
	}
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		if *verbose {
			log.Fatalf("System '%s' not found: %v", *sysName, err)
		}
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	// 2. Resolve LPAR UUID and Details from LPAR Name 
	if *verbose {
		fmt.Printf("Resolving LPAR UUID for '%s'...\n", *lparName)
	}
	lpar, partUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose)
	if err != nil || partUUID == "" {
		if *verbose {
			log.Fatalf("LPAR '%s' not found: %v", *lparName, err)
		}
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}
	
	if *verbose {
		fmt.Printf("✅ Found LPAR UUID: %s\n", partUUID)
		fmt.Printf("🔍 Current LPAR State: %s\n", lpar.PartitionState)
	}

	// -> NEW: Check the state before proceeding
	if lpar.PartitionState == "not activated" {
		// Even if not verbose, it's good practice to let the user know why we skipped
		fmt.Printf("⚠️ LPAR '%s' is already powered off ('not activated'). Skipping Power Off.\n", *lparName)
		return
	}

	// =========================================================================
	// EXECUTE POWER OFF
	// =========================================================================
	if *verbose {
		fmt.Println("Initiating Power Off...")
	}
	status, err := restClient.PowerOffPartition(ctx,partUUID, *shutdownOpt, *restart, *verbose)
	if err != nil {
		if *verbose {
			log.Fatalf("Failed to power off partition: %v", err)
		}
		log.Fatal("❌ Failed to power off partition.")
	}
	
	// This will print the status seamlessly.
	fmt.Printf("🚀 PowerOff Job Status: %s\n", status)
}