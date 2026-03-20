package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/beevik/etree"
	hmc "github.com/sudeeshjohn/powerhmc-go"
)

// Helper function to safely extract text from an etree Element
func safeGetText(elem *etree.Element, path string) string {
	if found := elem.FindElement(path); found != nil {
		return found.Text()
	}
	return "N/A"
}

func main() {
	// --- Configuration ---
	hmcIP      := "192.0.2.1"
	username   := "REDACTED_HMC_USER<=="
	password   := "REDACTED_HMC_PASS<=="
	sysName    := "LTC09U31-ZZ"
	verbose    := false 

	// 1. Initialize & Login
	restClient := hmc.NewHmcRestClient(hmcIP)
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer restClient.Logoff()

	// 2. Resolve System UUID
	fmt.Printf("Locating Managed System: %s...\n", sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(sysName, verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("Could not find system: %v", err)
	}

	// 3. Fetch Advanced Partitions XML
	fmt.Println("Downloading Advanced Partition configurations (this may take a moment)...")
	partitions, err := restClient.GetLogicalPartitionsAdv(sysUUID, verbose)
	if err != nil {
		log.Fatalf("Failed to retrieve advanced XML: %v", err)
	}

	fmt.Printf("\nFound %d Partitions. Extracting deep configurations:\n", len(partitions))
	fmt.Println(strings.Repeat("=", 80))

	// 4. Iterate through the slice of *etree.Element
	for _, lpar := range partitions {
		// Because xmlStripNamespace was used, we can query tags directly without prefixes
		name  := safeGetText(lpar, "PartitionName")
		id    := safeGetText(lpar, "PartitionID")
		state := safeGetText(lpar, "PartitionState")

		// Deep Extraction: Memory
		desiredMem := safeGetText(lpar, "PartitionMemoryConfiguration/DesiredMemory")
		maxMem     := safeGetText(lpar, "PartitionMemoryConfiguration/MaximumMemory")

		// Deep Extraction: Processors
		// Note: We check SharingMode to know where to look for CPU stats
		sharingMode := safeGetText(lpar, "PartitionProcessorConfiguration/SharingMode")
		var desiredCPU string
		if sharingMode == "keep idle procs" || sharingMode == "share idle procs" {
			desiredCPU = safeGetText(lpar, "PartitionProcessorConfiguration/DedicatedProcessorConfiguration/DesiredProcessors")
		} else {
			desiredCPU = safeGetText(lpar, "PartitionProcessorConfiguration/SharedProcessorConfiguration/DesiredVirtualProcessors")
		}

		// Deep Extraction: Boot Device
		bootDevice := safeGetText(lpar, "BootListInformation/BootDeviceList")

		// Print the extracted data
		fmt.Printf("LPAR: %s (ID: %s) - State: %s\n", name, id, state)
		fmt.Printf("  ├─ Memory (Desired / Max): %s MB / %s MB\n", desiredMem, maxMem)
		fmt.Printf("  ├─ Processor Config:       %s CPUs (%s)\n", desiredCPU, sharingMode)
		fmt.Printf("  └─ Boot Device:            %s\n", bootDevice)
		fmt.Println(strings.Repeat("-", 80))
	}
}
