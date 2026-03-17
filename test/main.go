package main

import (
	"fmt"
	"log"

	"github.com/sudeeshjohn/svc-go-sdk" // use correct import path
)

func main() {
	client := svc.NewClient("10.130.32.11", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	systemInfo, err := client.Lssystem()
	if err != nil {
		log.Fatalf("lssystem error: %v", err)
	}
	fmt.Printf("System: %+v\n", systemInfo)

	if err := client.Mkhost(svc.Host{Name: "host1", Fcwwpn: []string{"21000024FF3C4D2E", "210100E08B251EE6", "210100F08C262EE7"}, Type: "generic", Protocol: "scsi"}); err != nil {
		log.Fatalf("Mkhost error: %v", err)
	}
}
