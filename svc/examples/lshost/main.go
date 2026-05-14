package main

import (
	"context"
	"flag"
	"os"
	"strings"

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

	// List all hosts
	client.Logger.Info("Fetching all hosts...")
	hosts, err := client.Lshost(ctx)
	if err != nil {
		client.Logger.Error("Lshost error", "error", err)
		os.Exit(1)
	}
	client.Logger.Info("Total hosts found", "count", len(hosts))

	// Search specific host
	targetHost := "ltc09u31-vios1"
	client.Logger.Info("Searching for specific host...", "target", targetHost)

	host, err := client.LshostByTarget(ctx,targetHost)
	if err != nil {
		if strings.Contains(err.Error(), "CMMVC5754E") {
			client.Logger.Warn("Host not found", "target", targetHost)
		} else {
			client.Logger.Error("LshostByTarget error", "error", err)
			os.Exit(1)
		}
	} else {
		client.Logger.Info("✅ Found Host", "name", host.Name, "id", host.ID)
		client.Logger.Debug("Host Details", "status", host.Status, "protocol", host.Protocol, "portset", host.PortsetName)
	}
}