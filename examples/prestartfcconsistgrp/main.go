package main

import (
	"context"
	"flag"
	"os"

	"github.com/sudeeshjohn/svc-go-sdk"
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

	group := svc.FlashCopyConsistGroupID{ID: "test_fcgrp"}

	client.Logger.Info("Preparing FlashCopy consistency group...", "id", group.ID)

	if err := client.Prestartfcconsistgrp(ctx,group); err != nil {
		client.Logger.Error("Prestartfcconsistgrp error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully prepared FlashCopy consistency group", "id", group.ID)
}