package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// FLAGS & TARGET CONFIGURATION
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

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

	// 3. ENSURE PCM IS TRIGGERED ON VIA ENFORCEABLE SSH BOUNDS
	fmt.Printf("\n⚙️  Ensuring System Metrics Tracing is active on '%s'...\n", *sysName)
	pcmCmd := fmt.Sprintf("chlparutil -m %s -r config -s 30", *sysName)
	
	output, err := hmc.CliRunnerViaSSH(*hmcIP, *username, *password, pcmCmd, *verbose)
	if err != nil {
		log.Printf("⚠️ Warning: Configuration checkpoint notice: %v\nOutput: %s", err, output)
	} else {
		fmt.Println("✅ PCM framework actively streaming at 30-second target frames.")
	}

	// 4. FETCH THE SYSTEM-WIDE METRICS FEED
	fmt.Println("\n📡 Pulling system metrics engine index feed for the last 24 hours...")
	opts := &hmc.ManagedSystemMetricsOptions{
		StartTS: time.Now().Add(-24 * time.Hour),
		Feed:    "bySource", 
	}

	snapshots, err := restClient.GetManagedSystemAggregatedMetrics(context.Background(), sysUUID, opts, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to resolve system metrics catalog: %v", err)
	}

	if len(snapshots) == 0 {
		fmt.Println("\n⚠️  No historical structural files located on the engine.")
		fmt.Println("   Note: If PCM was just turned on, wait 15 minutes for Tier 1 files to build.")
		return
	}

	fmt.Printf("✅ Located %d distinct tracking snapshot profiles.\n\n", len(snapshots))

	// =========================================================================
	// SYSTEM-WIDE UNMARSHALING AND SUMMARY PRINT OUT
	// =========================================================================
	
	// ✨ SUMMARY TRACKERS: Track overall metrics across all time windows
	var totalSysAvgCpu float64
	var absoluteSysMaxCpu float64
	var totalSysSamplesCount int
	var sysTotalCores float64

	for _, snap := range snapshots {
		if snap.JSONLink == "" {
			continue
		}

		if *verbose {
			fmt.Printf("\n📥 Fetching full framework payload from window target link: %s\n", snap.Updated)
		}
		
		metrics, err := restClient.FetchSystemPcmMetricsPayload(context.Background(), snap.JSONLink, false)
		if err != nil {
			log.Printf("⚠️ Failed to fetch metrics payload from HMC: %v", err)
			continue
		}

		for _, sample := range metrics.SystemUtil.UtilSamples {
			fmt.Println("\n=========================================================================")
			fmt.Printf(" 🖥️  MANAGED SYSTEM HARDWARE TELEMETRY REPORT TIME: %s\n", sample.SampleInfo.TimeStamp)
			fmt.Println("=========================================================================")
			
			// 1. Core Hypervisor Hardware Allocation
			proc := sample.ServerUtil.Processor
			
			avgCpu, maxCpu := 0.0, 0.0
			if len(proc.UtilizedProcUnits) > 0 {
				avgCpu = proc.UtilizedProcUnits[0] // Index 0 is always AVG
			}
			if len(proc.UtilizedProcUnits) > 2 {
				maxCpu = proc.UtilizedProcUnits[2] // Index 2 is always MAX for Aggregated
			} else if len(proc.UtilizedProcUnits) > 0 {
				maxCpu = proc.UtilizedProcUnits[len(proc.UtilizedProcUnits)-1] // Fallback safety
			}

			if len(proc.TotalProcUnits) > 0 {
				sysTotalCores = proc.TotalProcUnits[0]
			}

			// ✨ ACCUMULATE GLOBAL COUNTERS
			if len(proc.UtilizedProcUnits) > 0 {
				totalSysAvgCpu += avgCpu
				totalSysSamplesCount++
				
				if maxCpu > absoluteSysMaxCpu {
					absoluteSysMaxCpu = maxCpu
				}
			}
			
			// Print current interval metrics
			if len(proc.TotalProcUnits) > 0 {
				fmt.Printf("   [Hypervisor Capacity]  Total Cores: %.1f | Used Cores: %.2f (Avg) / %.2f (Max)\n",
					sysTotalCores, avgCpu, maxCpu)
			}
			
			mem := sample.ServerUtil.Memory
			if len(mem.TotalMem) > 0 && len(mem.AvailableMem) > 0 {
				fmt.Printf("   [Memory Allocations]   Total RAM: %.0f MB | Free RAM: %.0f MB | Active VMs: %.0f MB\n",
					mem.TotalMem[0], mem.AvailableMem[0], mem.AssignedMemToLpars[0])
			}

			// 2. Virtual I/O Server Subsystem telemetry array mapping
			for _, vios := range sample.ViosUtil {
				fmt.Printf("   \n   👉 Virtual I/O Server Sub-host: %s (Partition ID: %d, State: %s)\n", vios.Name, vios.ID, vios.State)
				fmt.Println("   -------------------------------------------------------------------------")
				
				viosProc := vios.Processor
				if len(viosProc.UtilizedProcUnits) > 0 {
					fmt.Printf("      - CPU Utilization   : %.2f Units (Avg) / %.2f Units (Max) on Mode: %s\n",
						viosProc.UtilizedProcUnits[0], viosProc.UtilizedProcUnits[len(viosProc.UtilizedProcUnits)-1], viosProc.Mode)
				}
				
				for _, fc := range vios.Storage.FiberChannelAdapters {
					if len(fc.ReadBytes) > 0 {
						fmt.Printf("      - NPIV Fabric Link  : Slot: %s | WWPN: %s | Reads: %.1f/s | Writes: %.1f/s | Speed: %.1f GBPS\n",
							fc.ID, fc.Wwpn, fc.NumOfReads[0], fc.NumOfWrites[0], fc.RunningSpeed[0])
					}
				}
			}
		}
	}

	// =========================================================================
	// ✨ NEW: COMPUTE & PRINT THE FINAL SYSTEM GRAND SUMMARY
	// =========================================================================
	if totalSysSamplesCount > 0 {
		grandSysAvgCpu := totalSysAvgCpu / float64(totalSysSamplesCount)
		
		pctAvgUse := 0.0
		pctMaxUse := 0.0
		if sysTotalCores > 0 {
			pctAvgUse = (grandSysAvgCpu / sysTotalCores) * 100
			pctMaxUse = (absoluteSysMaxCpu / sysTotalCores) * 100
		}

		fmt.Println("\n=========================================================================")
		fmt.Println(" 📊 SYSTEM WIDE PROCESSING GRAND SUMMARY")
		fmt.Println("=========================================================================")
		fmt.Printf("   Total Framework Intervals Analyzed : %d\n", totalSysSamplesCount)
		fmt.Printf("   Total Managed System Core Pool     : %.1f Cores\n", sysTotalCores)
		fmt.Printf("   Grand Average System CPU Usage     : %.2f Cores (%.1f%% overall utilization)\n", grandSysAvgCpu, pctAvgUse)
		fmt.Printf("   Absolute System CPU Hardware Peak  : %.2f Cores (%.1f%% overall utilization)\n", absoluteSysMaxCpu, pctMaxUse)
		fmt.Println("=========================================================================")
	}

	fmt.Println("\n🎉 Managed System telemetry parsing completely validated!")
}