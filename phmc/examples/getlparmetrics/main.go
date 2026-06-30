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
	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

// =========================================================================
// IBM PCM JSON DATA STRUCTURES
// =========================================================================

type PcmMetricsPayload struct {
    SystemUtil struct {
        UtilInfo    UtilInfo     `json:"utilInfo"`
        UtilSamples []UtilSample `json:"utilSamples"`
    } `json:"systemUtil"`
}

type UtilInfo struct {
    MetricType       string   `json:"metricType"`
    Frequency        int      `json:"frequency"`
    StartTimeStamp   string   `json:"startTimeStamp"`
    EndTimeStamp     string   `json:"endTimeStamp"`
    MetricArrayOrder []string `json:"metricArrayOrder"`
}

type UtilSample struct {
    SampleInfo struct {
        TimeStamp              string `json:"timeStamp"`
        NumOfSamplesAggregated int    `json:"numOfSamplesAggregated"`
        Status                 int    `json:"status"`
    } `json:"sampleInfo"`
    LparsUtil []struct {
        Name      string           `json:"name"`
        State     string           `json:"state"`
        OSType    string           `json:"osType"`
        Memory    MemoryMetrics    `json:"memory"`
        Processor ProcessorMetrics `json:"processor"`
    } `json:"lparsUtil"`
}

type MemoryMetrics struct {
    LogicalMem []float64 `json:"logicalMem"` // Mapped to MetricArrayOrder [avg, min, max]
}

type ProcessorMetrics struct {
    Mode              string    `json:"mode"`
    UtilizedProcUnits []float64 `json:"utilizedProcUnits"` // Mapped to MetricArrayOrder [avg, min, max]
    EntitledProcUnits []float64 `json:"entitledProcUnits"` // Mapped to MetricArrayOrder [avg, min, max]
}

func main() {
    // =========================================================================
    // CONFIGURATION & FLAGS
    // =========================================================================
    hmcIP := flag.String("hmc-ip", "", "HMC IP address")
    username := flag.String("hmc-user", "", "HMC username")
    password := flag.String("hmc-pass", "", "HMC password")
    sysName := flag.String("system-name", "", "Managed System Name")
    lparName := flag.String("lpar-name", "", "Target LPAR Name")
    
    // ✨ NEW: Time Travel Flags
    startDateStr := flag.String("start-date", "", "Start date (YYYY-MM-DD) e.g., 2026-05-18")
    endDateStr := flag.String("end-date", "", "End date (YYYY-MM-DD) e.g., 2026-05-22")
    
    verbose := flag.Bool("verbose", false, "Enable verbose output")
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
    flag.Parse()

    // Validation
    if *password == "" || *sysName == "" || *lparName == "" {
        log.Fatal("❌ Error: hmc-pass, system-name, and lpar-name are required.")
    }

    // =========================================================================
    // ✨ NEW: DYNAMIC TIME WINDOW PARSING
    // =========================================================================
    var startTime, endTime time.Time
    var err error

    if *startDateStr != "" {
        startTime, err = time.Parse("2006-01-02", *startDateStr)
        if err != nil {
            log.Fatalf("❌ Invalid start-date format: %v", err)
        }
    } else {
        // Default to 24 hours ago if no start date is provided
        startTime = time.Now().Add(-24 * time.Hour)
    }

    if *endDateStr != "" {
        endTime, err = time.Parse("2006-01-02", *endDateStr)
        if err != nil {
            log.Fatalf("❌ Invalid end-date format: %v", err)
        }
        // Push to the very end of the specified day (23:59:59)
        endTime = endTime.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
    } else {
        // Default to current time if no end date is provided
        endTime = time.Now()
    }

    // =========================================================================
    // AUTHENTICATION
    // =========================================================================
    restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)

    if *verbose {
        log.Printf("Attempting to log on to HMC at %s with username %s", *hmcIP, *username)
    }
    if err := restClient.Login(context.Background(), *username, *password); err != nil {
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
    // DYNAMIC RESOLUTION
    // =========================================================================
    if *verbose {
        fmt.Printf("\nResolving System UUID for '%s'...\n", *sysName)
    }
    _, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName)
    if err != nil || sysUUID == "" {
        log.Fatalf("❌ System '%s' not found.", *sysName)
    }

    if *verbose {
        fmt.Printf("Resolving LPAR UUID for '%s'...\n", *lparName)
    }
    _, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName)
    if err != nil || lparUUID == "" {
        log.Fatalf("❌ LPAR '%s' not found.", *lparName)
    }

    // =========================================================================
    // FETCH ATOM FEED FOR AGGREGATED METRICS
    // =========================================================================
    fmt.Printf("\n📡 Fetching PCM Aggregated Metrics for window: %s to %s...\n", 
        startTime.Format("2006-01-02"), endTime.Format("2006-01-02"))
    
    opts := &hmc.AggregatedMetricsOptions{
        StartTS: startTime, 
        EndTS:   endTime,
    }

    // Ensure PCM Data Collection is enabled for the Managed System
    fmt.Printf("\n⚙️  Ensuring PCM data collection is enabled on '%s'...\n", *sysName)
    pcmCmd := fmt.Sprintf("chlparutil -m %s -r config -s 30", *sysName)
    
    output, err := hmc.CliRunnerViaSSH(*hmcIP, *username, *password, pcmCmd)
    if err != nil {
        log.Printf("⚠️ Warning: Failed to enable PCM via CLI: %v\nOutput: %s", err, output)
    } else {
        fmt.Println("✅ PCM data collection is enabled at a 30-second sample rate.")
    }

    // 1. Get the Atom feed links
    snapshots, err := restClient.GetLparAggregatedMetrics(context.Background(), sysUUID, lparUUID, opts)

    if len(snapshots) == 0 {
        fmt.Println("\n⚠️  No metrics snapshots found for this time period.")
        fmt.Println("   Ensure Performance and Capacity Monitoring (PCM) is enabled on the HMC for this system.")
        return
    }

    fmt.Printf("✅ Found %d metric snapshot(s).\n\n", len(snapshots))

    // =========================================================================
    // DOWNLOAD & PARSE JSON DATA
    // =========================================================================

    w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
    fmt.Fprintln(w, "TIMESTAMP\tCPU MODE\tALLOC UNITS\tAVG CPU USED\tMAX CPU USED\t% AVG USE\t% MAX USE\tALLOC MEM (MB)")
    fmt.Fprintln(w, "---------\t--------\t-----------\t------------\t------------\t---------\t---------\t--------------")

    // Summary trackers to compute the grand totals/averages across all periods
    var totalAvgCpu float64
    var absoluteMaxCpu float64
    var totalPctAvgCpu float64
    var absoluteMaxPctCpu float64
    var totalSamplesCount int

    // ✨ NEW: Trackers for allocated CPU and Memory to display in the summary
    var latestAllocCpu float64
    var latestAllocMem float64

    // 2. Loop and download the rich struct data via the SDK
    for _, snap := range snapshots {
        metrics, err := restClient.FetchPcmMetricsPayload(context.Background(), snap.JSONLink)
        if err != nil {
            log.Printf("Skipping snapshot %s: %v", snap.Published, err)
            continue
        }

        for _, sample := range metrics.SystemUtil.UtilSamples {
            for _, lpar := range sample.LparsUtil {

                // 1. Allocated Processor (Entitled Processing Units)
                allocCpu := 0.0
                if len(lpar.Processor.EntitledProcUnits) > 0 {
                    allocCpu = lpar.Processor.EntitledProcUnits[0]
                    latestAllocCpu = allocCpu // Update latest allocation tracker
                }

                // 2. Processor Usage (AVG and MAX)
                avgCpu, maxCpu := 0.0, 0.0
                if len(lpar.Processor.UtilizedProcUnits) > 0 {
                    avgCpu = lpar.Processor.UtilizedProcUnits[0] // Index 0 = AVG
                }
                if len(lpar.Processor.UtilizedProcUnits) > 2 {
                    maxCpu = lpar.Processor.UtilizedProcUnits[2] // Index 2 = MAX
                }

                // 3. Dynamically Calculate Utilization Percentages
                pctAvgCpu := 0.0
                if allocCpu > 0 {
                    pctAvgCpu = (avgCpu / allocCpu) * 100
                }

                pctMaxCpu := 0.0
                if allocCpu > 0 {
                    pctMaxCpu = (maxCpu / allocCpu) * 100
                }

                // 4. Allocated Memory
                allocMem := 0.0
                if len(lpar.Memory.LogicalMem) > 0 {
                    allocMem = lpar.Memory.LogicalMem[0]
                    latestAllocMem = allocMem // Update latest memory tracker
                }

                // Accumulate running metrics for the summary
                totalAvgCpu += avgCpu
                totalPctAvgCpu += pctAvgCpu
                totalSamplesCount++

                if maxCpu > absoluteMaxCpu {
                    absoluteMaxCpu = maxCpu
                }
                if pctMaxCpu > absoluteMaxPctCpu {
                    absoluteMaxPctCpu = pctMaxCpu
                }

                // Print the row cleanly into the tabwriter columns
                fmt.Fprintf(w, "%s\t%s\t%.2f Units\t%.2f Units\t%.2f Units\t%.1f%%\t%.1f%%\t%.0f MB\n",
                    sample.SampleInfo.TimeStamp,
                    lpar.Processor.Mode,
                    allocCpu,
                    avgCpu,
                    maxCpu,
                    pctAvgCpu,
                    pctMaxCpu,
                    allocMem,
                )
            }
        }
    }

    // Flush the intervals table
    w.Flush()

    // Compute and print the final Grand Summary block
    if totalSamplesCount > 0 {
        grandAvgCpu := totalAvgCpu / float64(totalSamplesCount)
        grandAvgPct := totalPctAvgCpu / float64(totalSamplesCount)

        fmt.Println("\n=========================================================================")
        fmt.Println(" 📊 GRAND SUMMARY (TOTAL PERIOD STATISTICS)")
        fmt.Println("=========================================================================")
        fmt.Printf("   Time Window Evaluated     : %s to %s\n", startTime.Format("2006-01-02"), endTime.Format("2006-01-02"))
        fmt.Printf("   Total Intervals Analyzed  : %d\n", totalSamplesCount)
        fmt.Printf("   Allocated Processing Units: %.2f Units (Latest)\n", latestAllocCpu)
        fmt.Printf("   Allocated Logical Memory  : %.0f MB (Latest)\n", latestAllocMem)
        fmt.Printf("   Grand Average CPU Usage   : %.2f Units (%.1f%% of entitlement)\n", grandAvgCpu, grandAvgPct)
        fmt.Printf("   Absolute Maximum CPU Peak : %.2f Units (%.1f%% of entitlement)\n", absoluteMaxCpu, absoluteMaxPctCpu)
        fmt.Println("=========================================================================")
    }

    fmt.Println("\n🎉 Successfully downloaded and parsed all JSON utilization data!")
}
