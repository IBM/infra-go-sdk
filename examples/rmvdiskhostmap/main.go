package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/sudeeshjohn/svc-go-sdk" // Adjust if your package path differs
)

func main() {
	// Initialize Client
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// Define your targets
	hostName := "ltc09u31-vios1"
	volName := "test_volume3" // Or "pvc-2.2.12" based on your earlier output

	fmt.Printf("Attempting to unmap volume '%s' from host '%s'...\n", volName, hostName)

	// Execute the unmap command
	err := client.Rmvdiskhostmap(hostName, volName)

	if err != nil {
		errStr := err.Error()
		// CMMVC6071E is the typical IBM error code for "The specified mapping does not exist"
		if strings.Contains(errStr, "CMMVC6071E") {
			fmt.Println("✅ Volume is already unmapped from this host. Nothing to do.")
		} else if strings.Contains(errStr, "CMMVC5754E") {
			fmt.Printf("❌ Error: Either the host '%s' or volume '%s' doesn't exist.\n", hostName, volName)
		} else {
			log.Fatalf("❌ Failed to unmap: %v\n", err)
		}
	} else {
		fmt.Printf("✅ Successfully unmapped volume '%s' from host '%s'!\n", volName, hostName)
	}
}
