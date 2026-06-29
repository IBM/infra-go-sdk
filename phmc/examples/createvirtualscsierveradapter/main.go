package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")

	sysName := flag.String("system-name", "", "Managed System Name")
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS Name")
	lparName := flag.String("lpar-name", "ocp-sno-lpar", "Target Client LPAR Name")
	
	// Explicitly defining both sides of the connection creates a clean topology
	viosSlot := flag.Int("vios-slot", 50, "Target Virtual Slot Number on the VIOS")
	clientSlot := flag.Int("client-slot", 50, "Target Virtual Slot Number on the Client LPAR")

	verbose := flag.Bool("verbose", true, "Enable verbose output")
	flag.Parse()
	_ = verbose

	if *password == "" || *sysName == "" || *viosName == "" || *lparName == "" || *viosSlot <= 0 || *clientSlot <= 0 {
		log.Fatal("❌ Error: hmc-pass, system-name, vios-name, lpar-name, and valid slot numbers are required.")
	}

	// =========================================================================
	// 1. AUTHENTICATION
	// =========================================================================
	fmt.Printf("🔌 Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// 2. DYNAMIC RESOLUTION (Name -> UUID & ID)
	// =========================================================================
	if *verbose {
		fmt.Printf("\n🔍 Resolving System '%s'...\n", *sysName)
	}
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found: %v", *sysName, err)
	}

	if *verbose {
		fmt.Printf("🔍 Resolving VIOS '%s' to UUID...\n", *viosName)
	}
	viosUUID, err := hmc.GetViosID(context.Background(), restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found: %v", *viosName, err)
	}

	if *verbose {
		fmt.Printf("🔍 Resolving LPAR '%s' to Partition ID...\n", *lparName)
	}
	lparDetails, _, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose)
	if err != nil || lparDetails == nil {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}
	clientLparID := lparDetails.PartitionID

	fmt.Printf("✅ Resolution complete. VIOS UUID: %s | Client LPAR ID: %d\n", viosUUID, clientLparID)

	// =========================================================================
	// 3. PROVISION SERVER ADAPTER
	// =========================================================================
	fmt.Printf("\n🚀 Provisioning vSCSI Server Adapter (vhost) on VIOS '%s' (Slot %d) pointing to LPAR ID %d (Slot %d)...\n", *viosName, *viosSlot, clientLparID, *clientSlot)
	
	adapterUUID, err := restClient.CreateVirtualSCSIServerAdapter(viosUUID, clientLparID, *viosSlot, *clientSlot, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed: %v", err)
	}

	fmt.Println("\n=========================================================================")
	fmt.Printf(" ✨ Success! Server Adapter Provisioned.\n")
	fmt.Printf("    Adapter UUID: %s\n", adapterUUID)
	fmt.Printf("    Topology: VIOS Slot %d <---> Client Slot %d\n", *viosSlot, *clientSlot)
	fmt.Println("=========================================================================")
}