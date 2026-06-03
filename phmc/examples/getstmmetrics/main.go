package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc" // Mapped to package path [cite: 1]
)

func main() {
	// =========================================================================
	// FLAGS & CONFIGURATION
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	lparName := flag.String("lpar-name", "", "Target LPAR Name to filter within PHYP (Optional)")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *sysName == "" {
		log.Fatal("❌ Error: hmc-pass and system-name are required.")
	}

	// 1. CONNECT & LOGON
	restClient := hmc.NewRestClient(*hmcIP) // Initializes insecure TLS client [cite: 128]
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil { // Authenticates with session token [cite: 106, 118]
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background()) // Flushes connections gracefully on exit [cite: 111, 117]

	// 2. RESOLVE SYSTEM & LPAR TARGETS
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose) // Resolves system UUID from JSON cache [cite: 1598]
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	var lparUUID string
	if *lparName != "" {
		_, resolvedUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose) // Resolves target LPAR [cite: 1595]
		if err != nil || resolvedUUID == "" {
			log.Printf("⚠️  Warning: Target LPAR '%s' could not be resolved. Skipping LPAR filter.", *lparName)
		} else {
			lparUUID = resolvedUUID
		}
	}

	// =========================================================================
	// 3. ENFORCE STM MONITOR PREFERENCES NATIVELY
	// =========================================================================
	fmt.Printf("\n⚙️  Verifying Short Term Monitor (STM) engine status for '%s'...\n", *sysName)
	prefs, err := restClient.GetManagedSystemPcmPreferences(context.Background(), sysUUID)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve PCM preferences: %v", err)
	}

	if !prefs.ShortTermMonitorEnabled {
		fmt.Println("   📡 STM engine is asleep. Waking it up natively...")
		prefs.ShortTermMonitorEnabled = true
		err = restClient.SetManagedSystemPcmPreferences(context.Background(), sysUUID, prefs)
		if err != nil {
			log.Fatalf("❌ Failed to enable STM: %v", err)
		}
		fmt.Println("   ✅ STM Engine enabled. (Sleeping 60s for 5-second buffers to generate...)")
		time.Sleep(60 * time.Second)
	} else {
		fmt.Println("   ✅ STM Engine is active.")
	}

	// =========================================================================
	// 4. FETCH THE 5-SECOND GRANULAR RAW METRICS FEED
	// =========================================================================
	fmt.Printf("\n📡 Querying RAW STM Metrics Feed...\n")
	opts := &hmc.ShortTermMetricsOptions{
		StartTS: time.Now().Add(-15 * time.Minute),
	}

	snapshots, err := restClient.GetShortTermMonitorMetrics(context.Background(), sysUUID, opts, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to resolve STM catalog: %v", err)
	}

	if len(snapshots) == 0 {
		fmt.Println("⚠️  No STM metrics located yet. Wait a few minutes and try again.")
		return
	}

	fmt.Printf("✅ Located %d distinct STM tracking snapshots.\n", len(snapshots))

	// Isolate the first available link for each classification group
	var phypLink, viosLink string
	for _, snap := range snapshots {
		if snap.JSONLink == "" {
			continue
		}
		if strings.Contains(snap.JSONLink, "_phyp_") && phypLink == "" {
			phypLink = snap.JSONLink
		} else if !strings.Contains(snap.JSONLink, "_phyp_") && viosLink == "" {
			viosLink = snap.JSONLink
		}
		if phypLink != "" && viosLink != "" {
			break
		}
	}

	// =========================================================================
	// 5. PROCESSING DATA STREAM 1: POWER HYPERVISOR (PHYP)
	// =========================================================================
	if phypLink != "" {
		fmt.Printf("\n⬇️  Downloading RAW [PHYP] Payload from: %s\n", phypLink)
		phypMetrics, err := restClient.FetchStmRawMetricsPayload(context.Background(), phypLink, *verbose)
		if err != nil {
			log.Printf("❌ Failed to parse PHYP metrics: %v", err)
		} else {
			sample := phypMetrics.SystemUtil.UtilSample
			fmt.Println("\n=========================================================================")
			fmt.Printf(" 🖥️  POWER HYPERVISOR SYSTEM OVERVIEW - %s\n", sample.TimeStamp)
			fmt.Println("=========================================================================")
			
			// Compute Resource Allocations
			fmt.Printf("   [CPU Hardware] Configurable Units: %.2f | Available for Assignment: %.2f | Frequency: %.0f Hz\n",
				sample.Processor.ConfigurableProcUnits, sample.Processor.AvailableProcUnits, sample.Processor.ProcCyclesPerSecond)
			fmt.Printf("   [RAM Hardware] Configurable Mem: %.0f MB | Available for Assignment: %.0f MB\n",
				sample.Memory.ConfigurableMem, sample.Memory.AvailableMem)
			fmt.Printf("   [Hypervisor]   Firmware Allocated CPU Cycles: %.0f | Assigned Memory Overhead: %.0f MB\n",
				sample.SystemFirmware.UtilizedProcCycles, sample.SystemFirmware.AssignedMem)

			// LPAR Processing Details if a target match is resolved
			if lparUUID != "" {
				for _, lpar := range sample.LparsUtil {
					if lpar.UUID == lparUUID {
						fmt.Println("\n   [TARGET LOGICAL PARTITION METRICS]")
						fmt.Printf("      LPAR Name / ID       : %s (ID: %d, Type: %s)\n", lpar.Name, lpar.ID, lpar.Type)
						fmt.Printf("      Running State / Mode : %s (%s, Affinity Score: %.0f/100)\n", lpar.State, lpar.Processor.Mode, lpar.AffinityScore)
						fmt.Printf("      Memory Space Bounds  : Logical Size: %.0f MB | Backed Physical: %.0f MB\n", lpar.Memory.LogicalMem, lpar.Memory.BackedPhysicalMem)
						fmt.Printf("      Entitled CPU Cycles  : %.0f Ticks\n", lpar.Processor.EntitledProcCycles)
						fmt.Printf("      Uncapped CPU Cycles  : %.0f Ticks\n", lpar.Processor.UtilizedUnCappedProcCycles)
						fmt.Printf("      Core Dispatch Delays : Spent %.0f cycles waiting across %.0f wait periods\n", lpar.Processor.TimeSpentWaitingForDispatch, lpar.Processor.NumOfTimesWaitedForProcessor)
					}
				}
			}
		}
	} else {
		fmt.Println("\n⚠️  No Hypervisor (PHYP) snapshot found inside this time window.")
	}

	// =========================================================================
	// 6. PROCESSING DATA STREAM 2: VIRTUAL I/O SERVER (VIOS)
	// =========================================================================
	if viosLink != "" {
		fmt.Printf("\n⬇️  Downloading RAW [VIOS] Payload from: %s\n", viosLink)
		viosMetrics, err := restClient.FetchStmRawViosMetricsPayload(context.Background(), viosLink, *verbose)
		if err != nil {
			log.Printf("❌ Failed to parse VIOS metrics: %v", err)
		} else {
			timestamp := viosMetrics.SystemUtil.UtilSample.TimeStamp

			for _, vios := range viosMetrics.SystemUtil.UtilSample.ViosUtil {
				fmt.Println("\n=========================================================================")
				fmt.Printf(" 💽 VIRTUAL I/O SERVER HARDWARE MAP [%s] - %s\n", vios.Name, timestamp)
				fmt.Println("=========================================================================")

				// Memory Pools and Swap spaces
				fmt.Printf("   [MEMORY HEALTH] Real Allocated Used: %.0f MB | Shared Buffers: %.0f MB | Active Swap Space: %.0f MB\n",
					vios.Memory.UtilizedMem, vios.Memory.UsedForNetworkBuffer, vios.Memory.SwapSpaceUsed)

				// Physical Fibre Channel Cards
				if len(vios.Storage.FiberChannelAdapters) > 0 {
					fmt.Println("\n   [PHYSICAL FIBRE CHANNEL ADAPTER HBA LOAD]")
					wFC := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
					fmt.Fprintln(wFC, "   LOCATION CODE\tWWPN\tLINK SPEED\tREADS\tWRITES\tREAD BYTES\tWRITE BYTES")
					fmt.Fprintln(wFC, "   -------------\t----\t----------\t-----\t------\t----------\t-----------")

					for _, fc := range vios.Storage.FiberChannelAdapters {
						fmt.Fprintf(wFC, "   %s\t%s\t%.0f GBPS\t%.0f\t%.0f\t%.0f\t%.0f\n",
							fc.PhysicalLocation, fc.WWPN, fc.RunningSpeed, fc.NumOfReads, fc.NumOfWrites, fc.ReadBytes, fc.WriteBytes)
					}
					wFC.Flush()
				}

				// Physical Storage Devices (Disk Queues)
				if len(vios.Storage.PhysicalDevices) > 0 {
					fmt.Println("\n   [PHYSICAL DISK STORAGE DEVICE QUEUES]")
					wDisk := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
					fmt.Fprintln(wDisk, "   DISK IDENTITY\tREAD REQ\tWRITE REQ\tREAD SERV\tWRITE SERV\tWAIT QUEUE\tQUEUE FULLS")
					fmt.Fprintln(wDisk, "   -------------\t--------\t---------\t---------\t----------\t----------\t-----------")

					for _, disk := range vios.Storage.PhysicalDevices {
						// Filter active backing hardware routes to avoid flooding stdout
						if disk.NumOfReads > 0 || disk.NumOfWrites > 0 {
							fmt.Fprintf(wDisk, "   %s\t%.0f\t%.0f\t%.2f ms\t%.2f ms\t%.0f\t%.0f\n",
								disk.ID, disk.NumOfReads, disk.NumOfWrites, disk.ReadServiceTime, disk.WriteServiceTime, disk.WaitQueueSize, disk.NumOfTimesServiceQueueIsFull)
						}
					}
					wDisk.Flush()
				}
			}
		}
	} else {
		fmt.Println("\n⚠️  No Virtual I/O Server (VIOS) snapshot found inside this time window.")
	}

	fmt.Println("\n🎉 Complete Short-Term Monitor (STM) infrastructure metrics parsed successfully!")
}