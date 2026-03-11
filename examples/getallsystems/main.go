package main

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	hmc "github.com/sudeeshjohn/PowerHMC" // Adjust to your package path
)

func main() {
	// --- Configuration ---
	hmcIP    := "192.0.2.1"
	username := "REDACTED_HMC_USER<=="
	password := "REDACTED_HMC_PASS<=="
	verbose  := false 

	// 1. Initialize and Login
	restClient := hmc.NewHmcRestClient(hmcIP)
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}

	defer restClient.Logoff()

	// 2. Fetch the Quick list
	systems, err := restClient.GetManagedSystemsQuick(verbose)
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
	
	// Table Header
	fmt.Fprintln(w, "NAME\tSTATE\tIP ADDRESS\tMTMS\tTYPE\tMEM(MB)\tCPU\tUUID")
	fmt.Fprintln(w, "----\t-----\t----------\t----\t----\t-------\t---\t----")

	for _, s := range systems {
		// Calculate memory in GB for readability if desired, 
		// but here we use MB as returned (ConfigurableSystemMemory)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%.1f\t%s\n",
			s.SystemName,
			s.State,
			s.IPAddress,
			s.MTMS,
			s.SystemType,
			s.InstalledSystemMemory/1024, // Showing in GB approximately
			s.InstalledSystemProcessors,
			s.UUID,
		)
	}
	w.Flush()

	// 4. Print Firmware Summary
	fmt.Println("\n--- Firmware Details ---")
	for _, s := range systems {
		fmt.Printf("[%s] Firmware: %s\n", s.SystemName, s.SystemFirmware)
	}
}