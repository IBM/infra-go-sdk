package main

import (
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
	logger := svc.NewDefaultLogger()

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		logger.Fatal("Usage: prestartfcmap -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}


	ctx := context.Background()
	
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()
	if *verbose {
		client = client.WithDebug()
	}

	if err := client.Authenticate(ctx); err != nil {
		client.Logger.Error("Authentication error", "error", err)
		os.Exit(1)
	}

	mapping := svc.FlashCopyMappingID{ID: "test_fcmap"}

	client.Logger.Info("Preparing FlashCopy mapping...", "id", mapping.ID)

	if err := client.Prestartfcmap(ctx,mapping); err != nil {
		client.Logger.Error("Prestartfcmap error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully prepared FlashCopy mapping", "id", mapping.ID)
}