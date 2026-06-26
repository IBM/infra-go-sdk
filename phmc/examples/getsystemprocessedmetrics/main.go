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
	// CONFIGURATION & FLAGS
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

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	// =========================================================================
	// FETCH SNAPSHOT CATALOG
	// =========================================================================
	fmt.Printf("\n📡 Querying Processed Metrics (30-sec intervals) for System '%s' over the last 15 minutes...\n", *sysName)
	
	opts := &hmc.AggregatedMetricsOptions{
		StartTS: time.Now().Add(-15 * time.Minute), 
	}

	snapshots, err := restClient.GetManagedSystemProcessedMetrics(context.Background(), sysUUID, opts, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to resolve processed metrics catalog: %v", err)
	}

	if len(snapshots) == 0 {
		fmt.Println("⚠️  No processed metrics files located. Ensure PCM engines are enabled!")
		return
	}

	fmt.Printf("✅ Located %d distinct 30-second tracking snapshots.\n\n", len(snapshots))

	// =========================================================================
	// DOWNLOAD & PARSE PAYLOADS VIA SDK
	// =========================================================================
	
	// ✨ NEW: Summary Trackers
	var (
		firstSampleTime string
		lastSampleTime  string
		sampleCount     int

		sysTotalCores   float64
		sumUsedCores    float64
		maxUsedCores    float64

		sumPhypCores    float64
		maxPhypCores    float64

		sysTotalMemGB   float64
		sumAvailMemGB   float64
		minAvailMemGB   float64 = -1.0 // Track minimum available to find the memory peak
	)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TIMESTAMP\tTOTAL CORES\tUSED CORES\tPHYP OVERHEAD\tTOTAL MEM (GB)\tAVAIL MEM (GB)")
	fmt.Fprintln(w, "---------\t-----------\t----------\t-------------\t--------------\t--------------")

	for _, snap := range snapshots {
		if snap.JSONLink == "" {
			continue
		}

		metrics, err := restClient.FetchSysProcessedMetricsPayload(context.Background(), snap.JSONLink, *verbose)
		if err != nil {
			log.Printf("⚠️ Failed to fetch payload for %s: %v", snap.Published, err)
			continue
		}

		for _, sample := range metrics.SystemUtil.UtilSamples {
			timestamp := sample.SampleInfo.TimeStamp

			if firstSampleTime == "" {
				firstSampleTime = timestamp
			}
			lastSampleTime = timestamp
			sampleCount++

			// Safely extract CPU stats
			totalCpu, usedCpu, phypCpu := 0.0, 0.0, 0.0
			if len(sample.ServerUtil.Processor.TotalProcUnits) > 0 {
				totalCpu = sample.ServerUtil.Processor.TotalProcUnits[0]
				sysTotalCores = totalCpu // Keep updating to hold latest hardware capacity
			}
			if len(sample.ServerUtil.Processor.UtilizedProcUnits) > 0 {
				usedCpu = sample.ServerUtil.Processor.UtilizedProcUnits[0]
			}
			if len(sample.SystemFirmwareUtil.UtilizedProcUnits) > 0 {
				phypCpu = sample.SystemFirmwareUtil.UtilizedProcUnits[0] // Hypervisor CPU load
			}

			// Safely extract Memory stats and convert from MB to GB
			totalMem, availMem := 0.0, 0.0
			if len(sample.ServerUtil.Memory.TotalMem) > 0 {
				totalMem = sample.ServerUtil.Memory.TotalMem[0] / 1024.0 
				sysTotalMemGB = totalMem // Keep updating to hold latest hardware capacity
			}
			if len(sample.ServerUtil.Memory.AvailableMem) > 0 {
				availMem = sample.ServerUtil.Memory.AvailableMem[0] / 1024.0 
			}

			// Accumulate running totals for the summary
			sumUsedCores += usedCpu
			if usedCpu > maxUsedCores {
				maxUsedCores = usedCpu
			}

			sumPhypCores += phypCpu
			if phypCpu > maxPhypCores {
				maxPhypCores = phypCpu
			}

			sumAvailMemGB += availMem
			if minAvailMemGB < 0 || availMem < minAvailMemGB {
				minAvailMemGB = availMem
			}

			fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%.2f\t%.1f\t%.1f\n",
				timestamp, totalCpu, usedCpu, phypCpu, totalMem, availMem)
		}
	}
	w.Flush()

	// =========================================================================
	// ✨ NEW: COMPUTE & PRINT THE FINAL SYSTEM GRAND SUMMARY
	// =========================================================================
	if sampleCount > 0 {
		avgUsedCores := sumUsedCores / float64(sampleCount)
		avgPhypCores := sumPhypCores / float64(sampleCount)
		
		avgUsedMemGB := sysTotalMemGB - (sumAvailMemGB / float64(sampleCount))
		maxUsedMemGB := sysTotalMemGB - minAvailMemGB // Peak memory usage

		pctAvgCpu := 0.0
		pctMaxCpu := 0.0
		if sysTotalCores > 0 {
			pctAvgCpu = (avgUsedCores / sysTotalCores) * 100
			pctMaxCpu = (maxUsedCores / sysTotalCores) * 100
		}

		pctAvgMem := 0.0
		pctMaxMem := 0.0
		if sysTotalMemGB > 0 {
			pctAvgMem = (avgUsedMemGB / sysTotalMemGB) * 100
			pctMaxMem = (maxUsedMemGB / sysTotalMemGB) * 100
		}

		fmt.Println("\n=========================================================================")
		fmt.Println(" 📊 MANAGED SYSTEM PROCESSED METRICS GRAND SUMMARY")
		fmt.Println("=========================================================================")
		fmt.Printf("   Evaluation Time Range              : %s to %s\n", firstSampleTime, lastSampleTime)
		fmt.Printf("   Total Framework Intervals Analyzed : %d\n", sampleCount)
		fmt.Println("   -------------------------------------------------------------------------")
		fmt.Printf("   Hardware CPU Capacity              : %.2f Cores\n", sysTotalCores)
		fmt.Printf("   Average Global CPU Usage           : %.2f Cores (%.1f%% of capacity)\n", avgUsedCores, pctAvgCpu)
		fmt.Printf("   Absolute Peak Global CPU           : %.2f Cores (%.1f%% of capacity)\n", maxUsedCores, pctMaxCpu)
		fmt.Println("   -------------------------------------------------------------------------")
		fmt.Printf("   Average Hypervisor (PHYP) Overhead : %.2f Cores\n", avgPhypCores)
		fmt.Printf("   Peak Hypervisor (PHYP) Overhead    : %.2f Cores\n", maxPhypCores)
		fmt.Println("   -------------------------------------------------------------------------")
		fmt.Printf("   Hardware Memory Capacity           : %.1f GB\n", sysTotalMemGB)
		fmt.Printf("   Average Memory Usage               : %.1f GB (%.1f%% of capacity)\n", avgUsedMemGB, pctAvgMem)
		fmt.Printf("   Absolute Peak Memory Usage         : %.1f GB (%.1f%% of capacity)\n", maxUsedMemGB, pctMaxMem)
		fmt.Println("=========================================================================")
	}

	fmt.Println("\n🎉 System metrics successfully parsed via the new SDK Unmarshaler!")
}