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

	mappingName := "test_fcmap"
	removeMapping := svc.FlashCopyMappingRemove{Force: true}

	client.Logger.Info("Deleting FlashCopy mapping...", "name", mappingName)

	if err := client.Rmfcmap(ctx,mappingName, removeMapping); err != nil {
		client.Logger.Error("Rmfcmap error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully deleted FlashCopy mapping", "name", mappingName)
}