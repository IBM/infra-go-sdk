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
		log.Fatal("Usage: mkfcmap -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()

	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}

	copyRate := 150
	grainSize := 256
	mapping := svc.FlashCopyMapping{
		Name:        "test_fcmap",
		Source:      "295",
		Target:      "224",
		CopyRate:    &copyRate,
		GrainSize:   &grainSize,
		Incremental: true,
		AutoDelete:  true,
	}

	log.Printf("Creating FlashCopy mapping...: name=%v", mapping.Name)

	if err := client.Mkfcmap(ctx,mapping); err != nil {
		log.Printf("Mkfcmap error: error=%v", err)
		os.Exit(1)
	}

	log.Printf("✅ Successfully created FlashCopy mapping: name=%v", mapping.Name)
}
