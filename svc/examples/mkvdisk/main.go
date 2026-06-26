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
		log.Fatal("Usage: mkvdisk -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
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

	grainSize := 256
	volume := svc.Volume{
		Name:       "test_volume2",
		MdiskGrp:   "0",
		Size:       120,
		Unit:       "gb",
		RSize:      "2%",
		Warning:    "80%",
		AutoExpand: true,
		GrainSize:  &grainSize,
	}

	client.Logger.Info("Creating new volume...", "volume_name", volume.Name)

	if err := client.Mkvdisk(ctx,volume); err != nil {
		client.Logger.Error("Mkvdisk error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully created disk", "volume_name", volume.Name)
}