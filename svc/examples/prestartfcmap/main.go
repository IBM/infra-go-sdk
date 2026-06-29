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
		log.Fatal("Usage: prestartfcmap -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}


	ctx := context.Background()
	
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}

	mapping := svc.FlashCopyMappingID{ID: "test_fcmap"}

	log.Printf("Preparing FlashCopy mapping...: id=%v", mapping.ID)

	if err := client.Prestartfcmap(ctx,mapping); err != nil {
		log.Printf("Prestartfcmap error: error=%v", err)
		os.Exit(1)
	}

	log.Printf("✅ Successfully prepared FlashCopy mapping: id=%v", mapping.ID)
}