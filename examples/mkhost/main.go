package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/sudeeshjohn/svc-go-sdk"
)

func main() {

	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	systemInfo, err := client.Lssystem()
	if err != nil {
		log.Fatalf("lssystem error: %v", err)
	}
	fmt.Printf("System: %+v\n", systemInfo)

	err = client.Mkhost(svc.Host{Name: "host1", Fcwwpn: []string{"21000024FF3C4D2E", "210100E08B251EE6", "210100F08C262EE7"}, Type: "generic", Protocol: "scsi"})
	if err != nil {
		if strings.Contains(err.Error(), "CMMVC6035E") || strings.Contains(err.Error(), "object already exists") {
			fmt.Println("Host already exists, skipping creation.")
		} else {
			log.Fatalf("Mkhost error: %v", err)
		}
	} else {
		fmt.Println("Successfully created host.")
	}
}
