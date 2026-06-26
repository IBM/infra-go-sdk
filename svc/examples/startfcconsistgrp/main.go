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
		logger.Fatal("Usage: startfcconsistgrp -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
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

	startGroup := svc.FlashCopyConsistGroupStart{
		ID:      "test_fcgrp",
		Prep:    true,
		Restore: true,
	}

	client.Logger.Info("Starting FlashCopy consistency group...", "id", startGroup.ID)

	if err := client.Startfcconsistgrp(ctx,startGroup); err != nil {
		client.Logger.Error("Startfcconsistgrp error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully started FlashCopy consistency group", "id", startGroup.ID)
}