package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	hmc "github.com/IBM/infra-go-sdk/phmc"
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	lparName := flag.String("lpar-name", "", "Target LPAR Name")
	
	// Boot Options
	keylock := flag.String("keylock", "normal", "Keylock position: normal, manual")
	bootMode := flag.String("boot-mode", "", "Boot mode: norm, dd, ds, of, sms (default: norm)")
	iiplSource := flag.String("iipl-source", "", "IBM i IPL source: a, b, c, or d")
	osType := flag.String("os-type", "", "OS type: OS400 (for IBM i IPL source)")
	
	// Network Boot Options (optional - for netboot mode)
	netboot := flag.Bool("netboot", false, "Enable network boot mode")
	macAddr := flag.String("mac", "", "MAC address of boot adapter (required for netboot)")
	clientIP := flag.String("client-ip", "", "Client IP address (for netboot)")
	serverIP := flag.String("server-ip", "", "Server IP address (for netboot)")
	gateway := flag.String("gateway", "", "Gateway address (for netboot)")
	netmask := flag.String("netmask", "", "Subnet mask (for netboot)")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()
	_ = verbose
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel() // Automatically cleans up the timer/goroutine the second the function exits

	// Validation
	if *password == "" || *sysName == "" || *lparName == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, and lpar-name are required.")
	}
	
	if *netboot && *macAddr == "" {
		log.Fatal("❌ Error: --mac is required when --netboot is enabled.")
	}

	fmt.Println("=========================================================================")
	fmt.Printf(" 🚀 PowerOn Partition: %s\n", *lparName)
	if *netboot {
		fmt.Println("    Mode: Network Boot")
		fmt.Printf("    MAC:  %s\n", *macAddr)
	} else {
		fmt.Println("    Mode: Normal Boot")
		fmt.Printf("    Keylock: %s\n", *keylock)
	}
	fmt.Println("=========================================================================")

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// DYNAMIC RESOLUTION & STATE CHECK
	// =========================================================================
	
	// 1. Resolve System UUID
	if *verbose {
		fmt.Printf("\n🔍 Resolving System UUID for '%s'...\n", *sysName)
	}
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found: %v", *sysName, err)
	}
	if *verbose {
		fmt.Printf("✅ System UUID: %s\n", sysUUID)
	}

	// 2. Resolve LPAR UUID and Check State
	if *verbose {
		fmt.Printf("🔍 Resolving LPAR UUID for '%s'...\n", *lparName)
	}
	lpar, partUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose)
	if err != nil || partUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found: %v", *lparName, err)
	}
	
	if *verbose {
		fmt.Printf("✅ LPAR UUID: %s\n", partUUID)
		fmt.Printf("📊 Current State: %s\n", lpar.PartitionState)
	}

	// Check if already running
	if lpar.PartitionState == "running" {
		fmt.Printf("⚠️  LPAR '%s' is already running. Skipping Power On.\n", *lparName)
		return
	}

	// 3. Get Profile UUID
	if *verbose {
		fmt.Println("🔍 Fetching partition profile...")
	}
	lparDetailed, err := restClient.GetLogicalPartitionDetailed(context.Background(), partUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve LPAR details: %v", err)
	}
	
	profileHref := lparDetailed.AssociatedPartitionProfile.Href
	if profileHref == "" {
		log.Fatal("❌ No associated partition profile found.")
	}
	profileUUID := profileHref[len(profileHref)-36:]
	
	if *verbose {
		fmt.Printf("✅ Profile: %s (UUID: %s)\n", lparDetailed.DefaultProfileName, profileUUID)
	}

	// =========================================================================
	// NETWORK BOOT: TRANSLATE MAC TO LOCATION CODE
	// =========================================================================
	var locationCode string
	var bootModeStr string
	
	if *netboot {
		fmt.Printf("\n🔍 Translating MAC %s to Location Code...\n", *macAddr)
		locationCode, err = restClient.GetLocationCodeByMac(context.Background(), sysUUID, partUUID, *macAddr, *verbose)
		if err != nil {
			log.Fatalf("❌ MAC translation failed: %v", err)
		}
		fmt.Printf("✅ Location Code: %s\n", locationCode)
		bootModeStr = "netboot"
	} else {
		bootModeStr = *bootMode
	}

	// =========================================================================
	// EXECUTE POWER ON
	// =========================================================================
	fmt.Println("\n🚀 Initiating Power On...")
	
	// Create PowerOnOptions
	options := &hmc.PowerOnOptions{
		ProfileUUID:  profileUUID,
		Keylock:      *keylock,
		IIPLSource:   *iiplSource,
		OSType:       *osType,
		BootMode:     bootModeStr,
		LocationCode: locationCode,
		ClientIP:     *clientIP,
		ServerIP:     *serverIP,
		Gateway:      *gateway,
		Netmask:      *netmask,
	}
	
	status, err := restClient.PowerOnPartition(ctx,partUUID, options, *verbose)
	
	if err != nil {
		log.Fatalf("❌ Power On failed: %v", err)
	}
	
	fmt.Printf("\n✅ Power On Complete! Status: %s\n", status)
	fmt.Println("=========================================================================")
}