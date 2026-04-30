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

	mappingName := "test_fcmap"
	client.Logger.Info("Verifying FlashCopy mapping...", "name", mappingName)

	mappings, err := client.Lsfcmap(ctx,mappingName)
	if err != nil || len(mappings) == 0 {
		client.Logger.Error("No FlashCopy mapping found", "name", mappingName)
		os.Exit(1)
	}

	startParams := svc.FlashCopyMappingStart{
		ID:      mappingName,
		Prep:    true,
		Restore: true,
	}

	client.Logger.Info("Starting FlashCopy mapping...", "id", startParams.ID)

	if err := client.Startfcmap(ctx,startParams); err != nil {
		client.Logger.Error("Startfcmap error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully started FlashCopy mapping", "id", startParams.ID)
}