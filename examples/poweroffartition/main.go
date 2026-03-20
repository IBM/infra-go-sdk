package main

import (
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	hmcIP := "192.0.2.3"
	username := "REDACTED_HMC_USER<=="
	password := "REDACTED_HMC_PASS<=="
	verbose := true
	partUUID := "6E20E53A-28F8-4D04-92D2-B32236C4B37A"

	// Initialize HmcRestClient
	restClient := hmc.NewHmcRestClient(hmcIP)

	// Perform Logon [cite: 489]
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer func() {
		if err := restClient.Logoff(); err != nil {
			log.Printf("Logoff failed: %v", err)
		} else if verbose {
			log.Println("Logged off successfully")
		}
	}()

	// Execute direct PowerOff [cite: 490]
	// Note: systemUUID parameter has been removed from the signature
	_, err := restClient.PowerOffPartition(partUUID, "Immediate", false, verbose)
	if err != nil {
		log.Fatalf("Power off failed: %v", err)
	}
	
	fmt.Printf("Successfully triggered Immediate PowerOff for Partition: %s\n", partUUID)
}