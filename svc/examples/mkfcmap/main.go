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
		logger.Fatal("Usage: mkfcmap -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
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

	client.Logger.Info("Creating FlashCopy mapping...", "name", mapping.Name)

	if err := client.Mkfcmap(ctx,mapping); err != nil {
		client.Logger.Error("Mkfcmap error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully created FlashCopy mapping", "name", mapping.Name)
}