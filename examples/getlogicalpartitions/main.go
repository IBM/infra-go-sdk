package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	
	flag.Parse()

	// Validation
	if *password == "" || *sysName == "" {
		log.Fatal("❌ Error: hmc-pass and system-name are required.")
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
	// DYNAMIC SYSTEM RESOLUTION
	// =========================================================================
	if *verbose {
		fmt.Printf("\nResolving System UUID for '%s'...\n", *sysName)
	}
	// Use the quick endpoint for fast system resolution
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		if *verbose {
			log.Fatalf("System '%s' not found: %v", *sysName, err)
		}
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	// =========================================================================
	// FETCH ADVANCED LPAR DATA (RETURNING STRUCTS)
	// =========================================================================
	if *verbose {
		fmt.Println("Downloading Advanced Partition configurations (this may take a moment)...")
	}
	
	partitions, err := restClient.GetLogicalPartitionsInSystem(sysUUID, *verbose)
	if err != nil {
		if *verbose {
			log.Fatalf("Failed to retrieve advanced configurations: %v", err)
		}
		log.Fatal("❌ Failed to retrieve advanced configurations.")
	}

	fmt.Printf("\n✅ Found %d Partitions on '%s'. Extracting configurations:\n", len(partitions), *sysName)
	fmt.Println("=======================================================================================================================================")

	// =========================================================================
	// ITERATE & DISPLAY IN A TABLE
	// =========================================================================
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	
	// Table Header
	fmt.Fprintln(w, "LPAR NAME\tUUID\tSTATE\tCPU (Units/vCPUs)\tMEMORY (Curr/Max MB)")
	fmt.Fprintln(w, "---------\t----\t-----\t-----------------\t--------------------")

	for _, lpar := range partitions {
		// Native Struct Access
		name := lpar.PartitionName
		uuid := lpar.PartitionUUID
		state := lpar.PartitionState

		// Deep Extraction: Memory
		currMem := lpar.PartitionMemoryConfiguration.CurrentMemory
		maxMem := lpar.PartitionMemoryConfiguration.MaximumMemory

		// Deep Extraction: Processors
		sharingMode := lpar.PartitionProcessorConfiguration.SharingMode
		var currProcUnits float64
		var currVcpus int
		
		if sharingMode == "keep idle procs" || sharingMode == "share idle procs" {
			// For dedicated processors
			currProcUnits = lpar.PartitionProcessorConfiguration.CurrentDedicatedProcessorConfiguration.CurrentProcessors
			// FIXED: Explicit cast from float64 to int
			currVcpus = int(lpar.PartitionProcessorConfiguration.CurrentDedicatedProcessorConfiguration.CurrentProcessors)
		} else {
			// For shared/uncapped processors
			currProcUnits = lpar.PartitionProcessorConfiguration.CurrentSharedProcessorConfiguration.CurrentProcessingUnits
			// CAST to int here to handle the float64 type safety update we made earlier!
			currVcpus = int(lpar.PartitionProcessorConfiguration.CurrentSharedProcessorConfiguration.AllocatedVirtualProcessors)
		}

		// Format complex columns
		cpuStr := fmt.Sprintf("%.1f / %d (%s)", currProcUnits, currVcpus, sharingMode)
		memStr := fmt.Sprintf("%.0f / %.0f", currMem, maxMem)

		// Print row into tabwriter
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, uuid, state, cpuStr, memStr)
	}

	w.Flush()
	fmt.Println("=======================================================================================================================================")
}