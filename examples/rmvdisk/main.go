package main

import (
	"context"
	"flag"
	"os"
	"strings"

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

	volumeName := "test_volume3"
	removeVolume := svc.VolumeRemove{Force: true, RemoveHostMappings: false}

	client.Logger.Info("Attempting to delete volume...", "volume", volumeName)

	if err := client.Rmvdisk(ctx,volumeName, removeVolume); err != nil {
		if strings.Contains(err.Error(), "CMMVC5754E") || strings.Contains(err.Error(), "CMMVC5804E") {
			client.Logger.Info("✅ Volume is already deleted (or does not exist). Nothing to do.", "volume", volumeName)
		} else {
			client.Logger.Error("Rmvdisk error", "error", err)
			os.Exit(1)
		}
	} else {
		client.Logger.Info("✅ Successfully deleted volume", "volume", volumeName)
	}
}