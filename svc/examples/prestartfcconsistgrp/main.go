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

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: prestartfcconsistgrp -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
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

	group := svc.FlashCopyConsistGroupID{ID: "test_fcgrp"}

	client.Logger.Info("Preparing FlashCopy consistency group...", "id", group.ID)

	if err := client.Prestartfcconsistgrp(ctx,group); err != nil {
		client.Logger.Error("Prestartfcconsistgrp error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully prepared FlashCopy consistency group", "id", group.ID)
}