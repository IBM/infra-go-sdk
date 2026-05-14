package main

import (
	"context"
	"flag"
	"os"

	"github.ibm.com/sudeeshjohn/infra-go-sdk/svc"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "REDACTED_SVC_IP<==", "SVC IP address")
	svcUser := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC username")
	svcPass := flag.String("svc-pass", "REDACTED_SVC_PASS<==", "SVC password")
	flag.Parse()


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