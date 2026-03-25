package main

import (
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	lparName := flag.String("lpar-name", "Go_LPAR_100", "Target LPAR Name")
	
	// Network Boot Parameters
	clientIP := flag.String("client-ip", "192.0.2.10", "IP to assign to the LPAR during boot")
	serverIP := flag.String("server-ip", "192.0.2.20", "IP of the Boot/NIM server")
	netmask  := flag.String("netmask", "255.255.240.0", "Subnet Mask")
	gateway  := flag.String("gateway", "192.0.2.254", "Network Gateway/Route")
	
	// Target MAC Address
	macAddr := flag.String("mac", "AA:BB:CC:DD:EE:FF", "REQUIRED: MAC Address of the boot adapter")
	
	verbose := flag.Bool("verbose", true, "Enable verbose output")
	flag.Parse()

	if *password == "" || *sysName == "" || *lparName == "" || *macAddr == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, lpar-name, and mac are required.")
	}

	fmt.Println("=========================================================================")
	fmt.Printf(" 🌐 Starting Static Network Boot for %s\n", *lparName)
	fmt.Printf("    - Client IP: %s\n    - Server IP: %s\n    - MAC:       %s\n", *clientIP, *serverIP, *macAddr)
	fmt.Println("=========================================================================")

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff()

	// 1. Resolve System
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found: %v", *sysName, err)
	}

	// 2. Resolve LPAR & Check State
	lpar, partUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || lpar == nil {
		log.Fatalf("❌ LPAR '%s' not found or could not be retrieved: %v", *lparName, err)
	}

	if lpar.PartitionState == "running" {
		log.Fatalf("❌ Error: LPAR '%s' is already running. You must power it off before initiating a netboot.", *lparName)
	}

	// 3. Resolve Profile UUID
	lparDetailed, err := restClient.GetLogicalPartitionDetailed(partUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve detailed LPAR information: %v", err)
	}
	profileHref := lparDetailed.AssociatedPartitionProfile.Href
	if profileHref == "" {
		log.Fatal("❌ No associated partition profile found for this LPAR.")
	}
	profileUUID := profileHref[len(profileHref)-36:] 

	// =========================================================================
	// TRANSLATE MAC TO LOCATION CODE
	// =========================================================================
	fmt.Printf("\n🔍 Translating MAC %s to Location Code...\n", *macAddr)
	
	locationCode, err := restClient.GetLocationCodeByMac(sysUUID, partUUID, *macAddr, *verbose)
	if err != nil {
		log.Fatalf("❌ Translation Failed: %v", err)
	}
	
	// Append -T1 to the location code for netboot
	//Note: Because Virtual Ethernet Adapters always have exactly one logical port, appending -T1.
	locationCode = locationCode + "-T1"
	
	if *verbose {
		fmt.Printf("✅ Found Location Code: %s\n", locationCode)
	}

	

	// =========================================================================
	// EXECUTE NETWORK POWER ON
	// =========================================================================
	fmt.Println("\n🚀 Sending Netboot Job to Hypervisor...")

	status, err := restClient.PowerOnPartition(
		partUUID, 
		profileUUID, 
		"normal", 
		"", 
		"", 
		"netboot",    
		locationCode, 
		*clientIP, 
		*serverIP, 
		*gateway, 
		*netmask, 
		*verbose,
	)

	if err != nil {
		log.Fatalf("❌ Failed to network boot partition: %v", err)
	}
	
	fmt.Printf("\n✅ Job Complete! Status: %s\n", status)
	fmt.Println("=========================================================================")
}
