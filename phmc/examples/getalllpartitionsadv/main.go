package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc"
)

func main() {
	hmcIP := flag.String("hmc-ip", "", "HMC IP")
	username := flag.String("hmc-user", "", "User")
	password := flag.String("hmc-pass", "", "Pass")
	sysName := flag.String("system-name", "", "System Name")
	verbose := flag.Bool("verbose", false, "Verbose")
	flag.Parse()

	client := hmc.NewHmcRestClient(*hmcIP)
	if err := client.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatal(err)
	}
	defer client.Logoff(context.Background())

	_, sysUUID, _ := client.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)

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