package main

import (
	"log"
	"context"
	"flag"
	"os"

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
		log.Fatal("Usage: startfcmap -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}


	ctx := context.Background()
	
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}

	mappingName := "test_fcmap"
	log.Printf("Verifying FlashCopy mapping...: name=%v", mappingName)

	mappings, err := client.Lsfcmap(ctx,mappingName)
	if err != nil || len(mappings) == 0 {
		log.Printf("No FlashCopy mapping found: name=%v", mappingName)
		os.Exit(1)
	}

	startParams := svc.FlashCopyMappingStart{
		ID:      mappingName,
		Prep:    true,
		Restore: true,
	}

	log.Printf("Starting FlashCopy mapping...: id=%v", startParams.ID)

	if err := client.Startfcmap(ctx,startParams); err != nil {
		log.Printf("Startfcmap error: error=%v", err)
		os.Exit(1)
	}

	log.Printf("✅ Successfully started FlashCopy mapping: id=%v", startParams.ID)
}
