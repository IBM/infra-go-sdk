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
    viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS")
    
    // Additional flags for storage mapping
    lparName := flag.String("lpar-name", "Go_LPAR_01", "Target LPAR Name")
    diskName := flag.String("disk-name", "hdisk3", "Name of the physical volume on the VIOS")
    
    verbose := flag.Bool("verbose", false, "Enable verbose output")
    flag.Parse()

    if *password == "" || *viosName == "" || *lparName == "" || *diskName == "" {
        log.Fatal("Error: hmc-pass, vios-name, lpar-name, and disk-name are required.")
    }

    // =========================================================================
    // AUTHENTICATION & RESOLUTION
    // =========================================================================
    restClient := hmc.NewHmcRestClient(*hmcIP)
    if err := restClient.Login(*username, *password, *verbose); err != nil {
        log.Fatalf("HMC Logon failed: %v", err)
    }
    defer restClient.Logoff()

    systems, _, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
    if err != nil || systems.UUID == "" {
        log.Fatalf("❌ System '%s' not found.", *sysName)
    }

    viosUUID, err := hmc.GetViosID(restClient, systems.UUID, *viosName, *verbose)
    if err != nil || viosUUID == "" {
        log.Fatalf("❌ VIOS '%s' not found.", *viosName)
    }

    // Note: Adjust this line to match your exact LPAR resolver function name 
    // (e.g., GetLogicalPartitionByName or hmc.GetLparID)
    lparUUID, err := restClient.GetLogicalPartitionByName(systems.UUID, *lparName, *verbose)
    if err != nil || lparUUID == "" {
        log.Fatalf("❌ LPAR '%s' not found.", *lparName)
    }

    // =========================================================================
    // EXECUTE STORAGE MAPPING
    // =========================================================================
    fmt.Printf("\n⚠️  Attempting to map Physical Volume '%s' from VIOS '%s' to LPAR '%s'...\n", *diskName, *viosName, *lparName)

    mappingUUID, err := restClient.CreatePhysicalVolumeMap(systems.UUID, viosUUID, lparUUID, *diskName, *verbose)
    if err != nil {
        log.Fatalf("❌ Storage Mapping Failed: %v", err)
    }

    fmt.Printf("\n💾 Successfully mapped Physical Volume. Mapping UUID: %s\n", mappingUUID)
}