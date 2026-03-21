package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "User")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "Pass")
	sysName := flag.String("system-name", "LTC13U05", "System Name")
	verbose := flag.Bool("verbose", false, "Verbose")
	flag.Parse()

	client := hmc.NewHmcRestClient(*hmcIP)
	if err := client.Login(*username, *password, *verbose); err != nil {
		log.Fatal(err)
	}
	defer client.Logoff()

	_, sysUUID, _ := client.GetManagedSystemByNameQuick(*sysName, *verbose)

	partitions, err := client.GetLogicalPartitionsAdv(sysUUID, *verbose)
	if err != nil {
		log.Fatal(err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	// Updated Header to match the printed columns exactly
	fmt.Fprintln(w, "NAME\tUUID\tSTATE\tCPU (U/V)\tMEM (CUR/MAX)\tOS VERSION")
	fmt.Fprintln(w, "----\t----\t-----\t---------\t-------------\t----------")
	
	for _, p := range partitions {
		var units float64
		var vcpus int
		conf := p.PartitionProcessorConfiguration

		if conf.SharingMode == "keep idle procs" || conf.SharingMode == "share idle procs" {
			// CurrentProcessors is float64 in types.go
			units = conf.CurrentDedicatedProcessorConfiguration.CurrentProcessors
			// FIXED: Added explicit int() cast here
			vcpus = int(conf.CurrentDedicatedProcessorConfiguration.CurrentProcessors)
		} else {
			units = conf.CurrentSharedProcessorConfiguration.CurrentProcessingUnits
			// FIXED: Added explicit int() cast here
			vcpus = int(conf.CurrentSharedProcessorConfiguration.AllocatedVirtualProcessors)
		}

		// Print to the table
		fmt.Fprintf(w, "%s\t%s\t%s\t%.1f/%d\t%.0f/%.0f\t%s\n",
			p.PartitionName,
			p.PartitionUUID,
			p.PartitionState,
			units, 
			vcpus,
			p.PartitionMemoryConfiguration.CurrentMemory,
			p.PartitionMemoryConfiguration.MaximumMemory,
			p.OperatingSystemVersion,
		)
	}
	w.Flush()
}