package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// FLAGS & CONFIGURATION
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	
	// Example toggle flags for bulk administration
	retentionDays := flag.Int("retention", 30, "Target global Aggregated Metrics Storage Duration (Days)")
	enforceAll := flag.Bool("enforce-all", true, "Automatically turn on LTM and Aggregation for ALL systems")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()
	_ = verbose

	if *password == "" {
		log.Fatal("❌ Error: hmc-pass is required.")
	}

	// 1. CONNECT & LOGON
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// 2. FETCH GLOBAL HMC PCM CONFIGURATION
	// =========================================================================
	fmt.Printf("\n⚙️  Fetching Global PCM Configuration for HMC at '%s'...\n", *hmcIP)
	
	prefs, err := restClient.GetManagementConsolePcmPreferences(context.Background(), *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve global PCM preferences: %v", err)
	}

	fmt.Println("\n=========================================================================")
	fmt.Println(" 📊 HMC GLOBAL PERFORMANCE & CAPACITY LIMITS")
	fmt.Println("=========================================================================")
	fmt.Printf("   Current Data Storage Retention Policy : %d Days\n", prefs.AggregatedMetricsStorageDuration)
	fmt.Println("   ---")
	fmt.Printf("   Maximum Systems allowed for LTM       : %d\n", prefs.MaximumManagedSystemsForLongTermMonitor)
	fmt.Printf("   Maximum Systems allowed for Compute   : %d\n", prefs.MaximumManagedSystemsForComputeLTM)
	fmt.Printf("   Maximum Systems allowed for Aggregat. : %d\n", prefs.MaximumManagedSystemsForAggregation)
	fmt.Printf("   Maximum Systems allowed for STM       : %d\n", prefs.MaximumManagedSystemsForShortTermMonitor)
	fmt.Println("=========================================================================")

	// =========================================================================
	// 3. DISPLAY MANAGED SYSTEMS STATUS IN A TABLE
	// =========================================================================
	fmt.Printf("\n📡 Processing Configuration for %d Managed Systems:\n", len(prefs.ManagedSystemPcmPreferences))
	
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SYSTEM NAME\tLTM ENABLED\tCOMPUTE LTM ENABLED\tAGGREGATION ENABLED\tENERGY CAPABLE")
	fmt.Fprintln(w, "-----------\t-----------\t-------------------\t-------------------\t--------------")

	needsUpdate := false

	// Update the retention day duration if it does not match our target
	if prefs.AggregatedMetricsStorageDuration != *retentionDays {
		prefs.AggregatedMetricsStorageDuration = *retentionDays
		needsUpdate = true
	}

	for i, sys := range prefs.ManagedSystemPcmPreferences {
		fmt.Fprintf(w, "%s\t%v\t%v\t%v\t%v\n",
			sys.SystemName,
			sys.LongTermMonitorEnabled,
			sys.ComputeLTMEnabled,
			sys.AggregationEnabled,
			sys.EnergyMonitoringCapable,
		)

		// ENFORCEMENT LOGIC: If a system is missing critical PCM flags, queue an update.
		if *enforceAll {
			if !sys.LongTermMonitorEnabled || !sys.ComputeLTMEnabled || !sys.AggregationEnabled {
				prefs.ManagedSystemPcmPreferences[i].LongTermMonitorEnabled = true
				prefs.ManagedSystemPcmPreferences[i].ComputeLTMEnabled = true
				prefs.ManagedSystemPcmPreferences[i].AggregationEnabled = true
				needsUpdate = true
			}
		}
	}
	w.Flush()

	// =========================================================================
	// 4. PUSH CHANGES BACK TO HMC IF NECESSARY
	// =========================================================================
	if needsUpdate {
		fmt.Println("\n⚠️  Configuration mismatch detected. Pushing bulk PCM configuration updates to HMC...")
		
		err = restClient.SetManagementConsolePcmPreferences(context.Background(), prefs, *verbose)
		if err != nil {
			log.Fatalf("❌ Failed to push global PCM configuration update: %v", err)
		}
		
		fmt.Printf("✅ Global PCM Preferences successfully updated! (Retention set to %d days)\n", *retentionDays)
	} else {
		fmt.Println("\n✅ All Managed Systems and Global Settings are already fully compliant. No updates needed.")
	}
}