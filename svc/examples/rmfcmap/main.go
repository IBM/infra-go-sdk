package main

import (
	"context"
	"flag"
	"os"

	"github.ibm.com/sudeeshjohn/infra-go-sdk/svc"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "", "SVC IP address (required)")
	svcUser := flag.String("svc-user", "", "SVC username (required)")
	svcPass := flag.String("svc-pass", "", "SVC password (required)")
	flag.Parse()

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: rmfcmap -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
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

	mappingName := "test_fcmap"
	removeMapping := svc.FlashCopyMappingRemove{Force: true}

	client.Logger.Info("Deleting FlashCopy mapping...", "name", mappingName)

	if err := client.Rmfcmap(ctx,mappingName, removeMapping); err != nil {
		client.Logger.Error("Rmfcmap error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully deleted FlashCopy mapping", "name", mappingName)
}