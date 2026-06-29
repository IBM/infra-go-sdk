package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
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
	lparName := flag.String("lpar-name", "", "Target LPAR Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()
	_ = verbose

	if *password == "" || *sysName == "" || *lparName == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, and lpar-name are required.")
	}

	// 1. CONNECT & LOGON
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// 2. RESOLVE SYSTEM & LPAR TARGETS
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	_, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}

	// =========================================================================
	// 3. ENFORCE PCM CONFIGURATION NATIVELY
	// =========================================================================
	fmt.Printf("\n⚙️  Verifying PCM Framework engines are running for '%s'...\n", *sysName)
	
	prefs, err := restClient.GetManagedSystemPcmPreferences(context.Background(), sysUUID)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve PCM preferences: %v", err)
	}

	needsUpdate := false

	if !prefs.LongTermMonitorEnabled { prefs.LongTermMonitorEnabled = true; needsUpdate = true }
	if !prefs.ComputeLTMEnabled { prefs.ComputeLTMEnabled = true; needsUpdate = true }
	if !prefs.AggregationEnabled { prefs.AggregationEnabled = true; needsUpdate = true }

	if needsUpdate {
		fmt.Println("   📡 Essential LTM tracking engines are disabled. Updating HMC configuration natively...")
		err = restClient.SetManagedSystemPcmPreferences(context.Background(), sysUUID, prefs)
		if err != nil {
			log.Fatalf("❌ Failed to push PCM configuration update: %v", err)
		}
		fmt.Println("   ✅ PCM Engines successfully turned on!")
	} else {
		fmt.Println("   ✅ PCM Configuration is active.")
	}

	// ✨ NEW: ENFORCE THE LPAR-LEVEL DATA COLLECTION LOCK
	fmt.Printf("\n⚙️  Verifying LPAR-level Performance Collection permissions for '%s'...\n", *lparName)
	err = restClient.EnableLparPerformanceCollection(context.Background(), lparUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to enable performance collection on LPAR: %v", err)
	}
	fmt.Println("   ✅ LPAR Performance Collection is Authorized.")

	// =========================================================================
	// 4. FETCH THE PROCESSED METRICS FEED (30-SECOND INTERVALS)
	// =========================================================================
	fmt.Printf("\n📡 Querying Processed Metrics (30-sec intervals) for LPAR '%s' over the last 60 minutes...\n", *lparName)
	
	opts := &hmc.LparProcessedMetricsOptions{
		StartTS: time.Now().Add(-60 * time.Minute), 
	}

	snapshots, err := restClient.GetLogicalPartitionProcessedMetrics(context.Background(), sysUUID, lparUUID, opts, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to resolve processed metrics catalog: %v", err)
	}

	if len(snapshots) == 0 {
		fmt.Println("⚠️  No processed metrics files located yet. The HMC daemon is still warming up. Run the script again in 3 minutes!")
		return
	}

	fmt.Printf("✅ Located %d distinct 30-second tracking snapshots.\n\n", len(snapshots))

	// =========================================================================
	// 5. UNMARSHAL AND DISPLAY METRICS
	// =========================================================================
	var totalLparCpu float64
	var maxLparCpu float64
	var sampleCount int
	var lparEntitled float64
	
	// ✨ NEW: Time Range Trackers
	var firstSampleTime string
	var lastSampleTime string

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	
	for _, snap := range snapshots {
		if snap.JSONLink == "" { continue }

		metrics, err := restClient.FetchLparProcessedMetricsPayload(context.Background(), snap.JSONLink)
		if err != nil {
			log.Printf("⚠️ Failed to fetch metrics payload from HMC: %v", err)
			continue
		}

		for _, sample := range metrics.SystemUtil.UtilSamples {
			
			// ✨ Update Time Trackers
			if firstSampleTime == "" {
				firstSampleTime = sample.SampleInfo.TimeStamp
			}
			lastSampleTime = sample.SampleInfo.TimeStamp

			for _, lpar := range sample.LparsUtil {
				
				// Extract Entitlement and Usage (Processed metrics only supply index [0] for AVG)
				allocCpu, usedCpu := 0.0, 0.0
				if len(lpar.Processor.EntitledProcUnits) > 0 { allocCpu = lpar.Processor.EntitledProcUnits[0] }
				if len(lpar.Processor.UtilizedProcUnits) > 0 { usedCpu = lpar.Processor.UtilizedProcUnits[0] }

				// ✨ Track globals for Final Summary
				if usedCpu > 0 {
					totalLparCpu += usedCpu
					sampleCount++
					lparEntitled = allocCpu // Keep updating to hold the latest entitlement boundary
					if usedCpu > maxLparCpu {
						maxLparCpu = usedCpu
					}
				}

				fmt.Println("=========================================================================")
				fmt.Printf(" 🖥️  PROCESSED LPAR METRICS REPORT TIME: %s\n", sample.SampleInfo.TimeStamp)
				fmt.Println("=========================================================================")

				// 1. Processor & Memory Overview
				fmt.Printf("   [Compute Status] OS: %s | State: %s | Mode: %s\n", lpar.OSType, lpar.State, lpar.Processor.Mode)
				fmt.Printf("   [CPU Telemetry]  Allocated: %.2f | Used: %.2f\n", allocCpu, usedCpu)

				if len(lpar.Memory.LogicalMem) > 0 {
					fmt.Printf("   [RAM Telemetry]  Logical Assigned: %.0f MB | Backed Physical: %.0f MB\n", 
						lpar.Memory.LogicalMem[0], lpar.Memory.BackedPhysicalMem[0])
				}

				// 2. Virtual Ethernet Network Load
				if len(lpar.Network.VirtualEthernetAdapters) > 0 {
					fmt.Println("\n   [Virtual Ethernet Adapters]")
					fmt.Fprintln(w, "   SLOT LOCATION\tVLAN ID\tTX BYTES/SEC\tRX BYTES/SEC")
					fmt.Fprintln(w, "   -------------\t-------\t------------\t------------")
					
					for _, veth := range lpar.Network.VirtualEthernetAdapters {
						tx, rx := 0.0, 0.0
						if len(veth.SentBytes) > 0 { tx = veth.SentBytes[0] }
						if len(veth.ReceivedBytes) > 0 { rx = veth.ReceivedBytes[0] }
						fmt.Fprintf(w, "   %s\t%d\t%.0f\t%.0f\n", veth.PhysicalLocation, veth.VlanID, tx, rx)
					}
					w.Flush()
				}

				// 3. NPIV (vFC) Storage Load
				if len(lpar.Storage.VirtualFiberChannelAdapters) > 0 {
					fmt.Println("\n   [Virtual Fibre Channel Storage Maps (NPIV)]")
					fmt.Fprintln(w, "   SLOT LOCATION\tCLIENT WWPN\tVIOS ID\tREAD BYTES/SEC\tWRITE BYTES/SEC\tLINK SPEED")
					fmt.Fprintln(w, "   -------------\t-----------\t-------\t--------------\t---------------\t----------")
					
					for _, vfc := range lpar.Storage.VirtualFiberChannelAdapters {
						read, write, speed := 0.0, 0.0, 0.0
						if len(vfc.ReadBytes) > 0 { read = vfc.ReadBytes[0] }
						if len(vfc.WriteBytes) > 0 { write = vfc.WriteBytes[0] }
						if len(vfc.RunningSpeed) > 0 { speed = vfc.RunningSpeed[0] }

						fmt.Fprintf(w, "   %s\t%s\t%d\t%.0f\t%.0f\t%.1f GBPS\n", 
							vfc.PhysicalLocation, vfc.WWPN, vfc.ViosID, read, write, speed)
					}
					w.Flush()
				}
				fmt.Println()
			}
		}
	}

	// =========================================================================
	// ✨ NEW: COMPUTE & PRINT THE FINAL LPAR GRAND SUMMARY
	// =========================================================================
	if sampleCount > 0 {
		avgLparCpu := totalLparCpu / float64(sampleCount)
		
		pctAvgUse, pctMaxUse := 0.0, 0.0
		if lparEntitled > 0 {
			pctAvgUse = (avgLparCpu / lparEntitled) * 100
			pctMaxUse = (maxLparCpu / lparEntitled) * 100
		}

		fmt.Println("\n=========================================================================")
		fmt.Println(" 📊 LPAR PROCESSED METRICS GRAND SUMMARY")
		fmt.Println("=========================================================================")
		fmt.Printf("   Evaluation Time Range              : %s to %s\n", firstSampleTime, lastSampleTime)
		fmt.Printf("   Total Framework Intervals Analyzed : %d\n", sampleCount)
		fmt.Printf("   LPAR Core Entitlement              : %.2f Cores\n", lparEntitled)
		fmt.Printf("   Average LPAR CPU Usage             : %.2f Cores (%.1f%% of entitlement)\n", avgLparCpu, pctAvgUse)
		fmt.Printf("   Absolute LPAR CPU Hardware Peak    : %.2f Cores (%.1f%% of entitlement)\n", maxLparCpu, pctMaxUse)
		fmt.Println("=========================================================================")
	}

	fmt.Println("\n🎉 LPAR Processed metrics parsed successfully!")
}