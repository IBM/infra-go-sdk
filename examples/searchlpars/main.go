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
	
	// Search parameters
	property := flag.String("property", "PartitionState", "Property to search by (e.g., PartitionState, PartitionType)")
	value := flag.String("value", "running", "Value to match (e.g., running, 'AIX/Linux')")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *property == "" || *value == "" {
		log.Fatal("❌ Error: hmc-pass, property, and value are required.")
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
	// EXECUTE SEARCH
	// =========================================================================
	fmt.Printf("\n🔍 Searching HMC for Partitions where %s == %s...\n", *property, *value)
	
	partitions, err := restClient.SearchLogicalPartitions(*property, *value, *verbose)
	if err != nil {
		log.Fatalf("❌ Search failed: %v", err)
	}

	if len(partitions) == 0 {
		fmt.Printf("⚠️  No partitions found matching %s == %s.\n", *property, *value)
		return
	}

	// =========================================================================
	// DISPLAY RESULTS IN A TABLE
	// =========================================================================
	fmt.Printf("✅ Found %d matching Partitions:\n", len(partitions))
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
