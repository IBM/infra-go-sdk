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

	if err := client.Mkfcmap(mapping); err != nil {
		client.Logger.Error("Mkfcmap error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully created FlashCopy mapping", "name", mapping.Name)
}