package main

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/IBM/infra-go-sdk/svc"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "", "SVC IP address (required)")
	svcUser := flag.String("svc-user", "", "SVC username (required)")
	svcPass := flag.String("svc-pass", "", "SVC password (required)")
	flag.Parse()

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: mkhost -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
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

	hostParams := svc.Host{
		Name:     "host1",
		Fcwwpn:   []string{"21000024FF3C4D2E", "210100E08B251EE6", "210100F08C262EE7"},
		Type:     "generic",
		Protocol: "scsi",
	}

	client.Logger.Info("Attempting to create host...", "host_name", hostParams.Name)

	err := client.Mkhost(ctx,hostParams)
	if err != nil {
		if strings.Contains(err.Error(), "CMMVC6035E") || strings.Contains(err.Error(), "object already exists") {
			client.Logger.Info("✅ Host already exists, skipping creation", "host_name", hostParams.Name)
		} else {
			client.Logger.Error("Mkhost error", "error", err)
			os.Exit(1)
		}
	} else {
		client.Logger.Info("✅ Successfully created host", "host_name", hostParams.Name)
	}
}