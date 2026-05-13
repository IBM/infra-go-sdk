package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"text/tabwriter"

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
	sysQuick, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
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

	// =========================================================================
	// CALCULATE RESOURCE USAGE
	// =========================================================================
	var totalCPU, totalMemMB float64
	for _, lpar := range partitions {
		// CPU calculation
		sharingMode := lpar.PartitionProcessorConfiguration.SharingMode
		if sharingMode == "keep idle procs" || sharingMode == "share idle procs" || sharingMode == "sre idle proces" {
			totalCPU += float64(lpar.PartitionProcessorConfiguration.CurrentDedicatedProcessorConfiguration.RunProcessors)
		} else {
			totalCPU += lpar.PartitionProcessorConfiguration.CurrentSharedProcessorConfiguration.CurrentProcessingUnits
		}
		// Memory calculation
		totalMemMB += lpar.PartitionMemoryConfiguration.CurrentMemory
	}
	
	totalMemGB := totalMemMB / 1024.0
	availCPU := sysQuick.CurrentAvailableSystemProcessorUnits
	availMemMB := float64(sysQuick.CurrentAvailableSystemMemory)
	availMemGB := availMemMB / 1024.0
	totalSystemCPU := sysQuick.ConfigurableSystemProcessorUnits
	totalSystemMemMB := float64(sysQuick.ConfigurableSystemMemory)
	totalSystemMemGB := totalSystemMemMB / 1024.0

	fmt.Printf("\n✅ Found %d Partitions on '%s'.\n", len(partitions), *sysName)
	fmt.Printf("\n📊 Resource Summary:\n")
	fmt.Printf("   CPU:    %.1f / %.1f used (%.1f available)\n", totalCPU, float64(totalSystemCPU), availCPU)
	fmt.Printf("   Memory: %.1f / %.1f GB used (%.1f GB available)\n", totalMemGB, totalSystemMemGB, availMemGB)
	
	if len(partitions) == 0 {
		fmt.Println("\nℹ️  No logical partitions exist on this system.")
		fmt.Println("   This is normal for a freshly configured system or one with all LPARs deleted.")
		return
	}

	fmt.Println("\nPartition Details:")
	fmt.Println("=======================================================================================================================================")

	// =========================================================================
	// SORT PARTITIONS BY NAME
	// =========================================================================
	sort.Slice(partitions, func(i, j int) bool {
		return partitions[i].PartitionName < partitions[j].PartitionName
	})

	// =========================================================================
	// ITERATE & DISPLAY IN A TABLE
	// =========================================================================
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	
	// Table Header
	fmt.Fprintln(w, "LPAR NAME\tUUID\tSTATE\tCPU (Units/vCPUs)\tMEMORY (Curr/Max GB)")
	fmt.Fprintln(w, "---------\t----\t-----\t------------------\t--------------------")

	for _, lpar := range partitions {
		// Native Struct Access
		name := lpar.PartitionName
		uuid := lpar.PartitionUUID
		state := lpar.PartitionState

		// Deep Extraction: Memory (convert MB to GB)
		currMemMB := lpar.PartitionMemoryConfiguration.CurrentMemory
		maxMemMB := lpar.PartitionMemoryConfiguration.MaximumMemory
		currMemGB := currMemMB / 1024.0
		maxMemGB := maxMemMB / 1024.0

		// Deep Extraction: Processors
		sharingMode := lpar.PartitionProcessorConfiguration.SharingMode
		var currProcUnits float64
		var currVcpus int
		
		if sharingMode == "keep idle procs" || sharingMode == "share idle procs" || sharingMode == "sre idle proces" {
			// For dedicated processors - use RunProcessors which shows actual allocated processors
			runProcs := lpar.PartitionProcessorConfiguration.CurrentDedicatedProcessorConfiguration.RunProcessors
			currProcUnits = float64(runProcs)
			currVcpus = runProcs
		} else {
			// For shared/uncapped processors
			currProcUnits = lpar.PartitionProcessorConfiguration.CurrentSharedProcessorConfiguration.CurrentProcessingUnits
			currVcpus = int(lpar.PartitionProcessorConfiguration.CurrentSharedProcessorConfiguration.AllocatedVirtualProcessors)
		}

		// Format complex columns
		cpuStr := fmt.Sprintf("%.1f / %d", currProcUnits, currVcpus)
		memStr := fmt.Sprintf("%.1f / %.1f", currMemGB, maxMemGB)

		// Print row into tabwriter
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, uuid, state, cpuStr, memStr)
	}

	w.Flush()
	fmt.Println("=======================================================================================================================================")
}