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
		log.Fatal("Usage: rmfcmap -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()

	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}

	mappingName := "test_fcmap"
	removeMapping := svc.FlashCopyMappingRemove{Force: true}

	log.Printf("Deleting FlashCopy mapping...: name=%v", mappingName)

	if err := client.Rmfcmap(ctx,mappingName, removeMapping); err != nil {
		log.Printf("Rmfcmap error: error=%v", err)
		os.Exit(1)
	}

	log.Printf("✅ Successfully deleted FlashCopy mapping: name=%v", mappingName)
}