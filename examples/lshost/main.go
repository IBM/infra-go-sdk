package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/mkumatag/svc-go-sdk" // Adjust if your package path differs
)

func main() {
	// Initialize Client
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// 1. List all hosts (General check)
	hosts, err := client.Lshost()
	if err != nil {
		log.Fatalf("Lshost error: %v", err)
	}
	fmt.Printf("Total hosts found: %d\n", len(hosts))

// 2. Specific Check for the host
    targetHost := "ltc13u29_vios1_2"
    fmt.Printf("Searching for host: %s...\n", targetHost)

    host, err := client.LshostByTarget(targetHost)
    if err != nil {
        if strings.Contains(err.Error(), "CMMVC5754E") {
            fmt.Printf("❌ Host '%s' not found (CMMVC5754E)\n", targetHost)
        } else {
            log.Fatalf("Error: %v", err)
        }
    } else {
        // 'host' is now a direct pointer to the object
        fmt.Printf("✅ Found Host: %s (ID: %s)\n", host.Name, host.ID)
        fmt.Printf("   Status: %s, Protocol: %s, Portset: %s\n", host.Status, host.Protocol, host.PortsetName)
    }

	fmt.Println("Done.")
}
