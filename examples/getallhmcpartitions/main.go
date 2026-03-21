package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/beevik/etree"
	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

// Helper function to safely extract text from an etree Element
func safeGetText(elem *etree.Element, path string) string {
	if found := elem.FindElement(path); found != nil {
		return found.Text()
	}
	return "N/A"
}

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	
	flag.Parse()

	if *password == "" {
		log.Fatal("❌ Error: hmc-pass is required.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	// =========================================================================
	// FETCH ALL LPARS ACROSS THE ENTIRE HMC
	// =========================================================================
	fmt.Printf("\n🌍 Querying HMC for ALL Managed Logical Partitions...\n")
	
	// Calling the newly renamed function
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
	fmt.Println("=====================================================================================================")
	
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "LPAR NAME\tID\tSTATE\tTYPE\tUUID")
	fmt.Fprintln(w, "---------\t--\t-----\t----\t----")

	for _, lpar := range partitions {
		name  := safeGetText(lpar, "PartitionName")
		id    := safeGetText(lpar, "PartitionID")
		state := safeGetText(lpar, "PartitionState")
		pType := safeGetText(lpar, "PartitionType")
		uuid  := safeGetText(lpar, "PartitionUUID")

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, id, state, pType, uuid)
	}
	
	w.Flush()
	fmt.Println("=====================================================================================================")
}
