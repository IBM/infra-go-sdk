package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc"
)

func main() {
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	viosName := flag.String("vios-name", "", "Target VIOS Name")
	showMAC := flag.Bool("show-mac", false, "Show VIOS trunk adapter MAC addresses instead of physical adapter inventory")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *hmcIP == "" || *username == "" || *password == "" || *sysName == "" || *viosName == "" {
		log.Fatal("❌ Error: hmc-ip, hmc-user, hmc-pass, system-name, and vios-name are required")
	}

	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ Login failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// 1. Resolve System
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found: %v", *sysName, err)
	}

	// 2. Resolve VIOS UUID using the VIOS quick inventory
	viosUUID, err := hmc.GetViosID(context.Background(), restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found on system '%s': %v", *viosName, *sysName, err)
	}

	if *showMAC {
		printVIOSMACCandidates(restClient, viosUUID, *viosName, *verbose)
		return
	}

	printVIOSPhysicalAdapterCandidates(restClient, sysUUID, viosUUID, *viosName, *verbose)
}

func printVIOSMACCandidates(restClient *hmc.RestClient, viosUUID, viosName string, verbose bool) {
	fmt.Printf("\n🚀 Fetching VIOS trunk adapters for '%s'...\n", viosName)
	fmt.Printf("✅ VIOS UUID: %s\n\n", viosUUID)

	viosDetails, err := restClient.GetVirtualIOServer(context.Background(), viosUUID, verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch VIOS details: %v", err)
	}

	if len(viosDetails.TrunkAdapters) == 0 {
		fmt.Printf("⚠️  No Virtual Ethernet Trunk Adapters found on VIOS '%s'\n", viosName)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "VIRTUAL SLOT\tMAC ADDRESS\tBASE LOCATION\tBOOT LOCATION (-T1)")
	fmt.Fprintln(w, "------------\t-----------\t-------------\t-------------------")

	count := 0
	for _, trunk := range viosDetails.TrunkAdapters {
		cleanMAC := strings.ReplaceAll(trunk.MACAddress, ":", "")
		formattedMAC := strings.ToUpper(hmc.FormatMACAddress(cleanMAC))

		locationCode := trunk.LocationCode
		if locationCode == "" {
			locationCode = "-"
		}

		netbootLocation := "-"
		if locationCode != "-" {
			netbootLocation = locationCode + "-T1"
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
			trunk.VirtualSlotNumber,
			formattedMAC,
			locationCode,
			netbootLocation,
		)
		count++
	}

	w.Flush()
	fmt.Println()
	fmt.Printf("✅ Retrieved %d VIOS trunk adapter(s)\n", count)
	fmt.Println("⚠️  These MAC addresses come from VIOS trunk adapters and are not HMC GetNetworkBootDevices results.")
}

func printVIOSPhysicalAdapterCandidates(restClient *hmc.RestClient, sysUUID, viosUUID, viosName string, verbose bool) {
	inventory, err := restClient.GetManagedSystem(context.Background(), sysUUID, verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch managed system inventory: %v", err)
	}

	fmt.Printf("\n🚀 Fetching VIOS-attached physical adapters for '%s'...\n", viosName)
	fmt.Printf("✅ VIOS UUID: %s\n\n", viosUUID)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "DEVICE NAME (LOC CODE)\tDRC INDEX\tDESCRIPTION\tBOOT LOCATION (-T0)")
	fmt.Fprintln(w, "----------------------\t---------\t-----------\t-------------------")

	matchedDevices := 0
	for _, bus := range inventory.IOConfig.IOBuses {
		for _, slot := range bus.IOSlots {
			if !strings.EqualFold(slot.PartitionName, viosName) {
				continue
			}

			adapter := slot.RelatedIOAdapter

			desc := adapter.Description
			if desc == "" || desc == "Empty slot" {
				desc = slot.Description
			}
			if desc == "" {
				desc = "N/A"
			}

			locCode := adapter.DeviceName
			if locCode == "" {
				locCode = slot.PhysicalLocationCode
			}
			if locCode == "" {
				locCode = "-"
			}

			drcIndex := slot.ConnectorIndex
			if drcIndex == "" {
				drcIndex = "-"
			}

			netbootLocation := "-"
			if locCode != "-" {
				netbootLocation = locCode + "-T0"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", locCode, drcIndex, desc, netbootLocation)
			matchedDevices++
		}
	}

	w.Flush()
	fmt.Println()

	if matchedDevices == 0 {
		fmt.Printf("⚠️  No physical adapters found assigned to VIOS '%s'\n", viosName)
		return
	}

	fmt.Printf("✅ Retrieved %d VIOS-attached physical adapter(s)\n", matchedDevices)
	fmt.Println("⚠️  These are candidate boot adapters inferred from VIOS inventory, not HMC GetNetworkBootDevices results.")
}

// Made with Bob
