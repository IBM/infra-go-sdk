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

	if err := client.Mkvdisk(volume); err != nil {
		client.Logger.Error("Mkvdisk error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ Successfully created disk", "volume_name", volume.Name)
}