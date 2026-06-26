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
		log.Fatal("Usage: mkfcconsistgrp -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
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

	group := svc.FlashCopyConsistGroup{
		Name:       "test_fcgrp",
		AutoDelete: false,
	}

	client.Logger.Info("Creating FlashCopy consistency group...", "name", group.Name)

	if err := client.Mkfcconsistgrp(ctx,group); err != nil {
		client.Logger.Error("Mkfcconsistgrp error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully created FlashCopy consistency group", "name", group.Name)
}