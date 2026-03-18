package main

import (
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/PowerHMC" // Adjust to your actual package path
)

func main() {
	// Flags
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP")
	user := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC User")
	pass := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC Password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Target Managed System")
	lparName := flag.String("lpar-name", "Go_LPAR_03", "Name for the new LPAR")
	verbose := flag.Bool("verbose", false, "Verbose logs")
	flag.Parse()

	// 1. Login
	client := hmc.NewHmcRestClient(*hmcIP)
	if err := client.Login(*user, *pass, *verbose); err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	defer client.Logoff()

	// 2. Resolve Managed System UUID
	fmt.Printf("🔍 Finding system %s...\n", *sysName)
	systems, _ := client.GetManagedSystemQuickAll(*verbose)
	var sysUUID string
	for _, s := range systems {
		if s.SystemName == *sysName {
			sysUUID = s.UUID
			break
		}
	}
	if sysUUID == "" {
		log.Fatalf("System %s not found.", *sysName)
	}

	// 3. Define Creation Request (0.5 CPU / 4GB RAM)
	req := hmc.CreateLparRequest{
		Name:             *lparName,
		MinMem:           2048,
		DesiredMem:       4096,
		MaxMem:           8192,
		MinProcUnits:     0.1,
		DesiredProcUnits: 0.5,
		MaxProcUnits:     2.0,
		MinVcpus:         1,
		DesiredVcpus:     1,
		MaxVcpus:         4,
		SharingMode:      "uncapped",
	}

	// 4. Execute Creation
	fmt.Printf("🚀 Provisioning LPAR '%s'...\n", *lparName)
	newUUID, err := client.CreateLogicalPartition(sysUUID, req, *verbose)
	if err != nil {
		log.Fatalf("❌ Creation failed: %v", err)
	}

	fmt.Printf("\n✅ Success!\nLPAR Name: %s\nLPAR UUID: %s\n", *lparName, newUUID)
}
