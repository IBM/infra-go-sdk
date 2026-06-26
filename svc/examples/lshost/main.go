package main

import (
	"log"
	"context"
	"flag"
	"os"
	"strings"

	"github.com/IBM/infra-go-sdk/svc"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "", "SVC IP address (required)")
	svcUser := flag.String("svc-user", "", "SVC username (required)")
	svcPass := flag.String("svc-pass", "", "SVC password (required)")
	flag.Parse()
	_ = verbose

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: lshost -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}

	// List all hosts
	log.Println("Fetching all hosts...")
	hosts, err := client.Lshost(ctx)
	if err != nil {
		log.Printf("Lshost error: error=%v", err)
		os.Exit(1)
	}
	log.Printf("Total hosts found: count=%v", len(hosts))

	// Search specific host
	targetHost := "ltc09u31-vios1"
	log.Printf("Searching for specific host...: target=%v", targetHost)

	host, err := client.LshostByTarget(ctx,targetHost)
	if err != nil {
		if strings.Contains(err.Error(), "CMMVC5754E") {
			log.Printf("Host not found: target=%v", targetHost)
		} else {
			log.Printf("LshostByTarget error: error=%v", err)
			os.Exit(1)
		}
	} else {
		log.Printf("✅ Found Host: name=%v id=%v", host.Name, host.ID)
		log.Printf("Host Details: status=%v protocol=%v portset=%v", host.Status, host.Protocol, host.PortsetName)
	}
}