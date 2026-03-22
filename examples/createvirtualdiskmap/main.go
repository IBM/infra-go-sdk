package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

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
    
    // Virtual Disk specific flags
    lparName := flag.String("lpar-name", "Go_LPAR_03", "Target LPAR Name")
    diskNamesStr := flag.String("disk-names", "auto_lv01", "Comma-separated list of Virtual Disks (LVs) on the VIOS (e.g., 'lv01,lv02,lv03')")
    
    verbose := flag.Bool("verbose", false, "Enable verbose output")
    flag.Parse()

    if *password == "" || *viosName == "" || *lparName == "" || *diskNamesStr == "" {
        log.Fatal("Error: hmc-pass, vios-name, lpar-name, and disk-names are required.")
    }

    // Parse comma-separated disk names
    diskNames := strings.Split(*diskNamesStr, ",")
    for i := range diskNames {
        diskNames[i] = strings.TrimSpace(diskNames[i])
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

    _,lparUUID, err := restClient.GetLogicalPartitionByName(systems.UUID, *lparName, *verbose)
    if err != nil || lparUUID == "" {
        log.Fatalf("❌ LPAR '%s' not found.", *lparName)
    }

    // =========================================================================
    // EXECUTE VIRTUAL DISK MAPPING (BATCH OPERATION)
    // =========================================================================
    fmt.Printf("\n⚠️  Attempting to map %d Virtual Disk(s) (LV) from VIOS '%s' to LPAR '%s'...\n", len(diskNames), *viosName, *lparName)
    fmt.Printf("Disks to map: %v\n", diskNames)

    mappingUUID, err := restClient.CreateVirtualDiskMaps(systems.UUID, viosUUID, lparUUID, diskNames, *verbose)
    if err != nil {
        log.Fatalf("❌ Storage Mapping Failed: %v", err)
    }

    fmt.Printf("\n💾 Successfully mapped %d Virtual Disk(s). Status: %s\n", len(diskNames), mappingUUID)
}
