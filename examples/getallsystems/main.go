package main

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your package path
)

func main() {
	// --- Configuration ---
	hmcIP := "192.0.2.1"
	username := "REDACTED_HMC_USER<=="
	password := "REDACTED_HMC_PASS<=="
	verbose := false

	// 1. Initialize and Login
	restClient := hmc.NewHmcRestClient(hmcIP)
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff()

	// 2. Fetch the Quick list
	systems, err := restClient.GetManagedSystemQuickAll(verbose)
	if err != nil {
		log.Fatalf("Error retrieving systems: %v", err)
	}

	if len(systems) == 0 {
		fmt.Println("No managed systems found.")
		return
	}

	// 3. Print all details using a tabwriter for clean columns
	fmt.Println("\n--- Managed Systems: Complete Quick Inventory ---")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Table Header (Updated MEM to GB for clarity)
	fmt.Fprintln(w, "NAME\tSTATE\tIP ADDRESS\tMTMS\tTYPE\tMEM(GB)\tCPU\tUUID")
	fmt.Fprintln(w, "----\t-----\t----------\t----\t----\t-------\t---\t----")

	for _, s := range systems {
		// Note: HMC reports InstalledSystemMemory in MB.
		// We use %.0f because the SDK uses float64 for scientific notation support.
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%.0f\t%.1f\t%s\n",
			s.SystemName,
			s.State,
			s.IPAddress,
			s.MTMS,
			s.SystemType,
			s.InstalledSystemMemory/1024,    // Approximate GB
			s.InstalledSystemProcessorUnits, // Corrected field name
			s.UUID,
		)
	}
	w.Flush()

	// 4. Print Firmware Summary
	fmt.Println("\n--- Firmware Details ---")
	for _, s := range systems {
		// Using ActivatedServicePackNameAndLevel provides more detail than just SystemFirmware
		fmt.Printf("[%s] Firmware: %s (Level: %s)\n", 
            s.SystemName, 
            s.SystemFirmware, 
            s.ActivatedLevel)
	}
}