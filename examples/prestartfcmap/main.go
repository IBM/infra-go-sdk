package main

import (
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

	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()
	if *verbose {
		client = client.WithDebug()
	}

	if err := client.Authenticate(); err != nil {
		client.Logger.Error("Authentication error", "error", err)
		os.Exit(1)
	}

	mapping := svc.FlashCopyMappingID{ID: "test_fcmap"}

	client.Logger.Info("Preparing FlashCopy mapping...", "id", mapping.ID)

	if err := client.Prestartfcmap(mapping); err != nil {
		client.Logger.Error("Prestartfcmap error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully prepared FlashCopy mapping", "id", mapping.ID)
}