package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	hmc "github.com/IBM/infra-go-sdk/phmc"
)

func main() {
	hmcIP    := flag.String("hmc-ip",   "", "HMC IP address or hostname (required)")
	username := flag.String("hmc-user", "", "HMC username (required)")
	password := flag.String("hmc-pass", "", "HMC password (required)")
	lparUUID := flag.String("lpar-uuid", "", "LPAR UUID (required)")
	viosID   := flag.Int("vios-id",   1,  "Target VIOS partition ID")
	viosSlot := flag.Int("vios-slot", 0,  "Available slot number on the target VIOS (required)")
	flag.Parse()

	if *hmcIP == "" || *username == "" || *password == "" || *lparUUID == "" || *viosSlot == 0 {
		log.Fatal("Usage: createvscsiclientadapter -hmc-ip <ip> -hmc-user <user> -hmc-pass <pass> -lpar-uuid <uuid> -vios-id <id> -vios-slot <slot>")
	}

	client := hmc.NewRestClient(*hmcIP)
	if err := client.Login(context.Background(), *username, *password, false); err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	defer client.Logoff(context.Background())

	fmt.Printf("Provisioning vSCSI Client Adapter mapped to VIOS %d, Slot %d...\n", *viosID, *viosSlot)
	adapterUUID, err := client.CreateVirtualSCSIClientAdapter(*lparUUID, *viosID, *viosSlot, true)
	if err != nil {
		log.Fatalf("Failed: %v", err)
	}

	fmt.Printf("Success! Adapter UUID: %s\n", adapterUUID)
}
