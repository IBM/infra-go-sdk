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
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	
	// Target System and LPAR
	sysName := flag.String("system-name", "", "Managed System Name")
	lparName := flag.String("lpar-name", "", "Target LPAR Name")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *sysName == "" || *lparName == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, and lpar-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// ⚡ LIGHTNING FAST RESOLUTION USING QUICK ENDPOINTS
	// =========================================================================
	
	// 1. Resolve System Name to UUID using GetManagedSystemQuickAll
	if *verbose { fmt.Printf("🔍 Resolving System '%s' (Quick)...\n", *sysName) }
	systems, err := restClient.GetManagedSystemQuickAll(context.Background(), *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch quick systems list: %v", err)
	}

	var sysUUID string
	for _, sys := range systems {
		if sys.SystemName == *sysName { 
			sysUUID = sys.UUID
			break
		}
	}
	if sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	// 2. Resolve LPAR Name to UUID using GetLogicalPartitionsQuickAll
	if *verbose { fmt.Printf("🔍 Resolving LPAR '%s' on %s (Quick)...\n", *lparName, sysUUID) }
	lpars, err := restClient.GetLogicalPartitionsQuickAll(context.Background(), sysUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch quick LPARs list: %v", err)
	}

	var lparUUID string
	for _, lpar := range lpars {
		if lpar.PartitionName == *lparName { 
			lparUUID = lpar.UUID
			break
		}
	}
	if lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found on system '%s'.", *lparName, *sysName)
	}

	// =========================================================================
	// FETCH ALL SPECIFIC PROPERTIES
	// =========================================================================
	fmt.Printf("\n📡 Querying all quick properties for LPAR '%s'...\n\n", *lparName)
	
	// List of all supported quick properties
	properties := []string{
		"IsVirtualServiceAttentionLEDOn",
		"MigrationState",
		"ProgressState",
		"PartitionType",
		"PartitionName",
		"PartitionID",
		"PartitionState",
		"RemoteRestartState",
		"AssociatedManagedSystem",
		"RMCState",
		"PowerManagementMode",
	}

	// Setup tabwriter for clean formatting
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PROPERTY\tVALUE")
	fmt.Fprintln(w, "--------\t-----")

	// Loop through and fetch each property individually
	for _, prop := range properties {
		value, err := restClient.GetLogicalPartitionQuickProperty(lparUUID, prop, *verbose)
		if err != nil {
			// Print the error in the table without crashing the script
			fmt.Fprintf(w, "%s\tERROR: %v\n", prop, err)
		} else {
			fmt.Fprintf(w, "%s\t%s\n", prop, value)
		}
	}

	// Flush the writer to standard output
	w.Flush()
	fmt.Println()
}