package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	
	flag.Parse()

	if *password == "" {
		log.Fatal("❌ Error: hmc-pass is required.")
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
	// FETCH ALL LPARS ACROSS THE ENTIRE HMC
	// =========================================================================
	fmt.Printf("\n🌍 Querying HMC for ALL Managed Logical Partitions...\n")
	
	// This now returns []hmc.LogicalPartitionDetailed
	partitions, err := restClient.GetAllLogicalPartitionsInHmc(*verbose)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve partitions: %v", err)
	}

	if len(partitions) == 0 {
		fmt.Println("No partitions found on this HMC.")
		return
	}

	// =========================================================================
	// DISPLAY RESULTS IN A TABLE
	// =========================================================================
	fmt.Printf("\n✅ Found %d Partitions globally across the HMC:\n", len(partitions))
	fmt.Println("========================================================================================================================")
	
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SYSTEM NAME\tLPAR NAME\tID\tSTATE\tTYPE\tUUID")
	fmt.Fprintln(w, "-----------\t---------\t--\t-----\t----\t----")

	for _, lpar := range partitions {
		// We access the struct fields DIRECTLY now. 
		// Note: PartitionID is an int, so we use %d in Fprintf.
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n", 
			lpar.SystemName,
			lpar.PartitionName, 
			lpar.PartitionID, 
			lpar.PartitionState, 
			lpar.PartitionType, 
			lpar.PartitionUUID,
		)
	}
	
	w.Flush()
	fmt.Println("========================================================================================================================")
}