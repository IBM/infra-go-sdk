package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// FLAGS & CONFIGURATION
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()
	_ = verbose

	if *password == "" || *sysName == "" {
		log.Fatal("❌ Error: hmc-pass and system-name are required.")
	}

	// 1. CONNECT & LOGON
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// 2. RESOLVE SYSTEM TARGET
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	// =========================================================================
	// 3. RETRIEVE AND ENFORCE PCM CONFIGURATION VIA REST
	// =========================================================================
	fmt.Printf("\n⚙️  Fetching current PCM Framework configuration for '%s'...\n", *sysName)
	
	prefs, err := restClient.GetManagedSystemPcmPreferences(context.Background(), sysUUID)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve PCM preferences: %v", err)
	}

	needsUpdate := false

	// Ensure Long Term Monitoring (LTM) and Compute LTM are active
	if !prefs.LongTermMonitorEnabled {
		prefs.LongTermMonitorEnabled = true
		needsUpdate = true
	}
	if !prefs.ComputeLTMEnabled {
		prefs.ComputeLTMEnabled = true
		needsUpdate = true
	}

	if needsUpdate {
		fmt.Println("   📡 Essential LTM tracking engines are disabled. Updating HMC configuration natively...")
		
		err = restClient.SetManagedSystemPcmPreferences(context.Background(), sysUUID, prefs)
		if err != nil {
			log.Fatalf("❌ Failed to push PCM configuration update: %v", err)
		}
		fmt.Println("   ✅ PCM Preferences successfully updated! (Note: It may take a few moments for data to buffer)")
	} else {
		fmt.Println("   ✅ PCM Configuration is already active and optimal. No updates needed.")
	}

	// =========================================================================
	// 4. FETCH THE RAW LONG TERM MONITOR FEED
	// =========================================================================
	fmt.Println("\n📡 Querying transient LTM Raw Metrics cache loop (30-min window)...")
	
	// Fetch last 5 minutes of raw LTM files to optimize performance
	opts := &hmc.LtmMetricsOptions{
		StartTS: time.Now().Add(-5 * time.Minute), 
	}

	snapshots, err := restClient.GetManagedSystemLtmFeed(context.Background(), sysUUID, opts)
	if err != nil {
		log.Fatalf("❌ Failed to query LTM metrics loop directory: %v", err)
	}

	if len(snapshots) == 0 {
		fmt.Println("⚠️  No raw real-time telemetry snapshots found. Retrying cycle...")
		return
	}

	fmt.Printf("✅ Discovered %d available LTM data files in the HMC retention buffer.\n", len(snapshots))

	// =========================================================================
	// ✨ NEW SUMMARY TRACKERS FOR RUNNING AVG & MAX CALCULATION
	// =========================================================================
	var totalSysUtilCores float64
	var absoluteSysMaxCores float64
	var sysTotalCores float64
	var ltmSampleCount int

	// =========================================================================
	// 5. DYNAMIC DISCRIMINATOR LOOP: EVALUATE CATEGORY AND PARSE RESPECTIVELY
	// =========================================================================
	for _, snap := range snapshots {
		if snap.JSONLink == "" {
			continue
		}

		categoryTag := strings.ToUpper(snap.Category)

		// ✨ ROUTE 1: POWER HYPERVISOR (PHYP) PAYLOADS
		if categoryTag == "PHYP" {
			metrics, err := restClient.FetchLtmPhypMetricsPayload(context.Background(), snap.JSONLink)
			if err != nil {
				log.Printf("⚠️ Failed to parse PHYP payload: %v", err)
				continue
			}
			
			sample := metrics.SystemUtil.UtilSample 
			proc := sample.Processor
			mem := sample.Memory

			// Calculate point-in-time utilization dynamically from the raw scalar fields
			currentUtilizedCores := proc.TotalProcUnits - proc.AvailableProcUnits
			sysTotalCores = proc.TotalProcUnits

			// Accumulate runtime summary metrics
			totalSysUtilCores += currentUtilizedCores
			ltmSampleCount++
			if currentUtilizedCores > absoluteSysMaxCores {
				absoluteSysMaxCores = currentUtilizedCores
			}
			
			fmt.Println("\n=========================================================================")
			fmt.Printf(" 👑 POWER HYPERVISOR RAW METRICS (SOURCE: %s) TIME: %s\n", snap.Category, sample.TimeStamp)
			fmt.Println("=========================================================================")
			
			// Print Firmware Core Load
			fmt.Printf("   -> Hypervisor Firmware Overhead CPU : %.0f Cycles Utilized\n", sample.SystemFirmware.UtilizedProcCycles)
			
			// Print System Wide Processor Scalars with dynamic active calculation
			fmt.Printf("   -> Total Physical Managed Core Pool : %.1f Cores Installed\n", proc.TotalProcUnits)
			fmt.Printf("   -> Configurable Core Pool Capacity  : %.2f Cores (Excluding Garded/Failed)\n", proc.ConfigurableProcUnits)
			fmt.Printf("   -> Active Core Allocation Load     : %.2f Cores Currently Utilized\n", currentUtilizedCores)
			fmt.Printf("   -> Available Unassigned Core Pools  : %.2f Cores Remaining Free\n", proc.AvailableProcUnits)

			// Print System Wide Memory Scalars
			fmt.Printf("   -> Total Physical Main Memory Pool  : %.0f MB Installed\n", mem.TotalMem)
			fmt.Printf("   -> Configurable Memory Boundaries   : %.0f MB\n", mem.ConfigurableMem)
			fmt.Printf("   -> Available Free Memory Pool       : %.0f MB\n", mem.AvailableMem)

		// ✨ ROUTE 2: VIRTUAL I/O SERVER (VIOS) PAYLOADS
		} else if strings.HasPrefix(strings.ToLower(snap.Category), "vios_") {
			metrics, err := restClient.FetchLtmViosMetricsPayload(context.Background(), snap.JSONLink)
			if err != nil {
				log.Printf("⚠️ Failed to parse VIOS payload: %v", err)
				continue
			}

			sample := metrics.SystemUtil.UtilSample 
			
			fmt.Println("\n   -------------------------------------------------------------------------")
			fmt.Printf("   v  VIRTUAL I/O SERVER LAYER METRICS (ATOM ENGINE SOURCE TAG: %s)\n", snap.Category)
			fmt.Println("   -------------------------------------------------------------------------")
			
			for _, vios := range sample.ViosUtil {
				fmt.Printf("      * Host Identifier Name    : %s (ID: %s)\n", vios.Name, vios.ID)
				
				// VIOS LTM Schema tracks active utilized memory
				fmt.Printf("      * Physical RAM Profile    : Internal Active Utilization: %.0f MB\n", vios.Memory.UtilizedMem)
				
				// Print direct Fibre Channel load (summing reads and writes to get total bytes transferred)
				for _, fc := range vios.Storage.FiberChannelAdapters {
					totalIOLoad := fc.ReadBytes + fc.WriteBytes
					fmt.Printf("      * NPIV Storage Adapter    : Port %s (WWPN: %s) | Rate: %.2f MB total I/O\n",
						fc.ID, fc.Wwpn, totalIOLoad/(1024*1024))
				}
			}
		}
	}

	// =========================================================================
	// ✨ COMPUTE & PRINT THE FINAL LTM HARDWARE GRAND SUMMARY
	// =========================================================================
	if ltmSampleCount > 0 {
		grandSysAvgCores := totalSysUtilCores / float64(ltmSampleCount)
		
		pctAvgUse := 0.0
		pctMaxUse := 0.0
		if sysTotalCores > 0 {
			pctAvgUse = (grandSysAvgCores / sysTotalCores) * 100
			pctMaxUse = (absoluteSysMaxCores / sysTotalCores) * 100
		}

		fmt.Println("\n=========================================================================")
		fmt.Println(" 📊 RAW MONITOR LTM PROCESSOR GRAND SUMMARY")
		fmt.Println("=========================================================================")
		fmt.Printf("   Total Raw Snapshots Evaluated     : %d\n", ltmSampleCount)
		fmt.Printf("   Total System Hardware Core Pool   : %.1f Cores\n", sysTotalCores)
		fmt.Printf("   Grand Average System CPU Usage    : %.2f Cores (%.1f%% overall utilization)\n", grandSysAvgCores, pctAvgUse)
		fmt.Printf("   Absolute System CPU Hardware Peak : %.2f Cores (%.1f%% overall utilization)\n", absoluteSysMaxCores, pctMaxUse)
		fmt.Println("=========================================================================")
	}

	fmt.Println("\n🎉 Long Term Monitor data streams parsed successfully!")
}